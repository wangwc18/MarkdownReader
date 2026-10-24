package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	win "golang.org/x/sys/windows"
)

func settingsObject(s Settings) (map[string]interface{}, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var object map[string]interface{}
	err = json.Unmarshal(b, &object)
	return object, err
}

// Atomic replacement means readers see either complete version; no write lock
// is needed for this read. Preserve any local delta that has not been persisted.
func (a *App) RefreshSettings() (*Settings, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, err := os.ReadFile(filepath.Join(a.dataDir, "settings.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var object map[string]interface{}
	if err = json.Unmarshal(b, &object); err != nil || object == nil {
		return nil, fmt.Errorf("配置文件损坏，原文件保留")
	}
	disk := NewApp(a.base).settings
	if err = json.Unmarshal(b, &disk); err != nil {
		return nil, err
	}
	latest, err := settingsObject(disk)
	if err != nil {
		return nil, err
	}
	if reflect.DeepEqual(latest, a.settingsBase) {
		return nil, nil
	}
	diskBase, _ := settingsObject(disk)
	local, err := settingsObject(a.settings)
	if err != nil {
		return nil, err
	}
	mergeSettings(latest, a.settingsBase, local)
	b, err = json.Marshal(latest)
	if err != nil {
		return nil, err
	}
	var merged Settings
	if err = json.Unmarshal(b, &merged); err != nil {
		return nil, err
	}
	a.settings, a.settingsBase = merged, diskBase
	snapshot := a.settingsSnapshot()
	return &snapshot, nil
}

// The frontend submits the snapshot it edited. Background file operations may
// already have refreshed this process's settings from another window.
func (a *App) SetSettingsFrom(s, baseline Settings) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	current, err := settingsObject(a.settings)
	if err != nil {
		return err
	}
	before, err := settingsObject(baseline)
	if err != nil {
		return err
	}
	local, err := settingsObject(s)
	if err != nil {
		return err
	}
	delete(before, "recent")
	delete(local, "recent")
	delete(before, "fileEncodings")
	delete(local, "fileEncodings")
	mergeSettings(current, before, local)
	b, err := json.Marshal(current)
	if err != nil {
		return err
	}
	var merged Settings
	if err = json.Unmarshal(b, &merged); err != nil {
		return err
	}
	return a.setSettings(merged)
}

// Apply only this instance's changes to the latest disk value. Nested maps
// merge per key, so independent shortcut and file-encoding changes survive.
func mergeSettings(target, base, local map[string]interface{}) {
	keys := map[string]bool{}
	for key := range base {
		keys[key] = true
	}
	for key := range local {
		keys[key] = true
	}
	for key := range keys {
		before, existed := base[key]
		next, exists := local[key]
		if existed == exists && reflect.DeepEqual(before, next) {
			continue
		}
		oldMap, wasMap := before.(map[string]interface{})
		newMap, isMap := next.(map[string]interface{})
		if isMap || (wasMap && !exists) {
			current, _ := target[key].(map[string]interface{})
			if current == nil {
				current = map[string]interface{}{}
			}
			mergeSettings(current, oldMap, newMap)
			target[key] = current
		} else if exists {
			target[key] = next
		} else {
			delete(target, key)
		}
	}
}

func mergeRecent(latest, base, local []string) []string {
	contains := func(paths []string, path string) bool {
		for _, p := range paths {
			if encodingPathKey(p) == encodingPathKey(path) {
				return true
			}
		}
		return false
	}
	result := []string{}
	// Local ordering is relevant only when this instance changed it.
	for _, p := range local {
		if !contains(result, p) {
			result = append(result, p)
		}
	}
	for _, p := range latest {
		if !contains(result, p) && (!contains(base, p) || contains(local, p)) {
			result = append(result, p)
		}
	}
	if len(result) > 30 {
		result = result[:30]
	}
	return result
}

func (a *App) persistSettings() error {
	lock, err := os.OpenFile(filepath.Join(a.dataDir, "settings.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	var overlapped win.Overlapped
	if err = win.LockFileEx(win.Handle(lock.Fd()), win.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		return err
	}
	defer win.UnlockFileEx(win.Handle(lock.Fd()), 0, 1, 0, &overlapped)
	local, err := settingsObject(a.settings)
	if err != nil {
		return err
	}
	latest, err := settingsObject(a.settings)
	if err != nil {
		return err
	}
	path := filepath.Join(a.dataDir, "settings.json")
	b, err := os.ReadFile(path)
	if err == nil {
		var object map[string]interface{}
		if err = json.Unmarshal(b, &object); err != nil || object == nil {
			return fmt.Errorf("配置文件损坏，原文件保留")
		}
		disk := NewApp(a.base).settings
		if err = json.Unmarshal(b, &disk); err != nil {
			return fmt.Errorf("配置文件损坏，原文件保留：%w", err)
		}
		latest, err = settingsObject(disk)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var baseSettings, latestSettings Settings
	baseBytes, _ := json.Marshal(a.settingsBase)
	_ = json.Unmarshal(baseBytes, &baseSettings)
	latestBytes, _ := json.Marshal(latest)
	_ = json.Unmarshal(latestBytes, &latestSettings)
	mergeSettings(latest, a.settingsBase, local)
	if !reflect.DeepEqual(a.settings.Recent, baseSettings.Recent) {
		latest["recent"] = mergeRecent(latestSettings.Recent, baseSettings.Recent, a.settings.Recent)
	}
	b, err = json.MarshalIndent(latest, "", "  ")
	if err != nil {
		return err
	}
	var merged Settings
	if err = json.Unmarshal(b, &merged); err != nil {
		return err
	}
	if err = validatePersonalWords(merged.PersonalWords); err != nil {
		return err
	}
	if err = atomicWrite(path, b); err != nil {
		return err
	}
	a.settings = merged
	a.settingsBase, _ = settingsObject(merged)
	return nil
}
