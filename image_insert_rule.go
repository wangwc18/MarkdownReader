package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.yaml.in/yaml/v3"
)

func documentImageInsertRule(content string) (string, error) {
	metadata, err := exportMetadata(content)
	if err != nil {
		return "", err
	}
	if metadata == nil {
		return "", nil
	}
	value, err := metadataLookup(metadata, []string{"typora-copy-images-to"}, 0)
	if err != nil {
		return "", err
	}
	if value == nil || value.Tag == "!!null" {
		return "", nil
	}
	if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
		return "", fmt.Errorf("typora-copy-images-to 必须是目录字符串或 upload")
	}
	return value.Value, nil
}

func documentImageCopyFolder(content, base string) (string, bool, error) {
	rule, err := documentImageInsertRule(content)
	if err != nil {
		return "", false, err
	}
	if rule == "" || rule == "upload" {
		return "", false, nil
	}
	path := filepath.FromSlash(rule)
	if !filepath.IsAbs(path) {
		if base == "" {
			return "", false, fmt.Errorf("请先保存文档，再设置图片复制目录")
		}
		path = filepath.Join(base, path)
	}
	return filepath.Clean(path), true, nil
}

func imageInsertionAction(content string, settings Settings) (string, error) {
	return imageInsertionActionFor(content, settings, false)
}

func imageInsertionActionFor(content string, settings Settings, remote bool) (string, error) {
	if remote && !settings.ApplyRemoteImageRules {
		return "none", nil
	}
	rule, err := documentImageInsertRule(content)
	if err != nil {
		return "", err
	}
	if rule == "upload" && settings.AllowYAMLImageUpload {
		return "upload", nil
	}
	if rule != "" && rule != "upload" {
		return "copy", nil
	}
	if !remote && !settings.ApplyLocalImageRules {
		return "none", nil
	}
	action := settings.ImageInsertAction
	if action == "" {
		if settings.CopyImages {
			return "copy", nil
		}
		return "none", nil
	}
	if action != "none" && action != "copy" && action != "upload" {
		return "", fmt.Errorf("无效的图片插入规则")
	}
	return action, nil
}

func (a *App) SetImageUploadRule(id string) (*ImageRootEdit, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.docs[id]
	if d == nil {
		return nil, fmt.Errorf("文档不存在")
	}
	content, err := withDocumentImageSetting(d.Content, "typora-copy-images-to", "upload")
	if err != nil {
		return nil, err
	}
	return &ImageRootEdit{Original: d.Content, Content: content}, nil
}

func insertedImageFolder(content, base, fallback, documentPath string, custom ...bool) (string, bool, error) {
	folder, configured, err := documentImageCopyFolder(content, base)
	if err != nil {
		return "", false, err
	}
	if configured {
		if err = os.MkdirAll(folder, 0755); err != nil {
			return "", false, err
		}
		return folder, true, nil
	}
	if strings.Contains(fallback, "${filename}") {
		if documentPath == "" {
			return "", false, fmt.Errorf("请先保存文档，再使用文件名图片目录")
		}
		name := filepath.Base(documentPath)
		fallback = strings.ReplaceAll(fallback, "${filename}", strings.TrimSuffix(name, filepath.Ext(name)))
	}
	folder, err = imageStorageFolder(base, fallback, custom...)
	return folder, false, err
}

func (a *App) ChooseGlobalImageFolder() (string, error) {
	a.mu.Lock()
	initial := a.settings.ImageFolder
	a.mu.Unlock()
	if !filepath.IsAbs(initial) {
		initial = a.base
	}
	if info, err := os.Stat(initial); err != nil || !info.IsDir() {
		initial = a.base
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择图片复制目录", DefaultDirectory: initial})
}

func (a *App) ChooseImageCopyFolder(id string) (*ImageRootEdit, error) {
	a.mu.Lock()
	d := a.docs[id]
	if d == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("文档不存在")
	}
	original, path := d.Content, d.Path
	a.mu.Unlock()
	if path == "" {
		return nil, fmt.Errorf("请先保存文档，再设置图片复制目录")
	}
	base := filepath.Dir(path)
	initial, _, err := documentImageCopyFolder(original, base)
	if err != nil {
		return nil, err
	}
	if info, e := os.Stat(initial); e != nil || !info.IsDir() {
		initial = base
	}
	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择图片存储路径", DefaultDirectory: initial})
	if err != nil || directory == "" {
		return nil, err
	}
	relative, err := filepath.Rel(base, directory)
	if err != nil {
		relative = directory
	}
	content, err := withDocumentImageSetting(original, "typora-copy-images-to", filepath.ToSlash(relative))
	if err != nil {
		return nil, err
	}
	return &ImageRootEdit{Original: original, Content: content}, nil
}
