package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	win "golang.org/x/sys/windows"
)

type DeletedFile struct {
	ID           string    `json:"id"`
	OriginalPath string    `json:"originalPath"`
	Hash         string    `json:"hash"`
	Mode         uint32    `json:"mode"`
	DeletedAt    time.Time `json:"deletedAt"`
	State        string    `json:"state"`
}

func validateDocumentName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || strings.HasSuffix(name, ".") || strings.ContainsAny(name, `<>:"/\|?*`) || strings.IndexFunc(name, unicode.IsControl) >= 0 || !isMarkdown(name) {
		return errors.New("请输入有效的 Markdown 或文本文件名，不能包含路径和保留字符")
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return errors.New("文件名是 Windows 保留名称")
	}
	return nil
}

func (a *App) RenameDocument(id, name string) (*Document, error) {
	if err := validateDocumentName(name); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}
	if d.Path == "" {
		return nil, errors.New("请先保存文档")
	}
	b, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	if digest(b) != d.Hash {
		return nil, errors.New("文件已被外部修改，请重新加载后重命名")
	}
	target := filepath.Join(filepath.Dir(d.Path), name)
	if target != d.Path {
		if info, e := os.Stat(target); e == nil {
			original, e := os.Stat(d.Path)
			if e != nil || !os.SameFile(original, info) {
				return nil, errors.New("同名文件已存在，不会覆盖")
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		from, _ := win.UTF16PtrFromString(d.Path)
		to, _ := win.UTF16PtrFromString(target)
		if err = win.MoveFileEx(from, to, win.MOVEFILE_WRITE_THROUGH); err != nil {
			return nil, err
		}
	}
	old := d.Path
	d.Path = target
	d.Name = name
	if encoding, ok := a.settings.FileEncodings[encodingPathKey(old)]; ok {
		delete(a.settings.FileEncodings, encodingPathKey(old))
		a.rememberEncoding(target, encoding)
	}
	for i, path := range a.settings.Recent {
		if strings.EqualFold(path, old) {
			a.settings.Recent[i] = target
		}
	}
	// File state must reach the editor even if auxiliary recent/history writes fail.
	_ = a.persistSettings()
	_ = a.writeRecovery()
	copy := *d
	return &copy, nil
}

func (a *App) deletionPath(id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", errors.New("无效的删除记录")
	}
	return filepath.Join(a.dataDir, "deleted", id), nil
}
func writeDeletion(dir string, record DeletedFile) error {
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "record.json"), b)
}
func (a *App) DeleteDocumentFile(id string) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}
	if d.Path == "" {
		return nil, errors.New("此文档尚未保存为文件")
	}
	info, err := os.Lstat(d.Path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("只能删除普通文档文件")
	}
	b, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	if digest(b) != d.Hash {
		return nil, errors.New("文件已被外部修改，请重新加载后删除")
	}
	record := DeletedFile{ID: uuid.NewString(), OriginalPath: d.Path, Hash: digest(b), Mode: uint32(info.Mode().Perm()), DeletedAt: time.Now().UTC(), State: "prepared"}
	dir, _ := a.deletionPath(record.ID)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err = atomicWrite(filepath.Join(dir, "content"), b); err != nil {
		return nil, err
	}
	if err = writeDeletion(dir, record); err != nil {
		return nil, err
	}
	// Recheck immediately before handing the exact, fully-qualified file to Windows.
	current, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	if digest(current) != record.Hash {
		return nil, errors.New("删除准备期间文件发生变化，已取消")
	}
	recycle := a.recycle
	if recycle == nil {
		recycle = recycleFile
	}
	if err = recycle(d.Path); err != nil {
		if _, statError := os.Lstat(d.Path); statError == nil {
			record.State = "failed"
			_ = writeDeletion(dir, record)
		}
		return nil, err
	}
	if _, err = os.Lstat(d.Path); !os.IsNotExist(err) {
		return nil, errors.New("系统未完成删除，编辑内容已保留")
	}
	record.State = "deleted"
	_ = writeDeletion(dir, record)
	old := d.Path
	d.Path = ""
	d.Hash = ""
	d.Dirty = true
	d.Detached = true
	paths := []string{}
	for _, path := range a.settings.Recent {
		if !strings.EqualFold(path, old) {
			paths = append(paths, path)
		}
	}
	a.settings.Recent = paths
	_ = a.persistSettings()
	_ = a.writeRecovery()
	copy := *d
	return &copy, nil
}

func (a *App) ListDeletedFiles() ([]DeletedFile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	root := filepath.Join(a.dataDir, "deleted")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []DeletedFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	list := []DeletedFile{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir, err := a.deletionPath(entry.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "record.json"))
		if err != nil {
			continue
		}
		var record DeletedFile
		if json.Unmarshal(b, &record) != nil || record.ID != entry.Name() {
			continue
		}
		_, e := os.Stat(record.OriginalPath)
		if record.State == "deleted" || record.State == "prepared" && os.IsNotExist(e) {
			list = append(list, record)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].DeletedAt.After(list[j].DeletedAt) })
	return list, nil
}

func createRestoredFile(path string, b []byte, mode os.FileMode) error {
	if !filepath.IsAbs(path) {
		return errors.New("恢复记录中的原文件路径无效")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".markdownreader-restore-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, mode); err != nil {
		return err
	}
	from, _ := win.UTF16PtrFromString(tmp)
	to, err := win.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// No REPLACE_EXISTING flag: an original path recreated by another app is preserved.
	return win.MoveFileEx(from, to, win.MOVEFILE_WRITE_THROUGH)
}
func (a *App) RestoreDeletedFile(id string) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	dir, err := a.deletionPath(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "record.json"))
	if err != nil {
		return nil, err
	}
	var record DeletedFile
	if err = json.Unmarshal(b, &record); err != nil {
		return nil, err
	}
	if record.ID != id || record.State == "restored" {
		return nil, errors.New("此删除记录已恢复或无效")
	}
	b, err = os.ReadFile(filepath.Join(dir, "content"))
	if err != nil {
		return nil, err
	}
	if digest(b) != record.Hash {
		return nil, errors.New("恢复副本校验失败，未写入原路径")
	}
	if err = createRestoredFile(record.OriginalPath, b, os.FileMode(record.Mode)); err != nil {
		return nil, fmt.Errorf("未恢复，不会覆盖同名文件：%w", err)
	}
	record.State = "restored"
	_ = writeDeletion(dir, record)
	return a.open(record.OriginalPath)
}
