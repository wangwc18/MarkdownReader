package main

import (
	"fmt"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) localImageMoveSource(id, reference string) (*imageCopySource, error) {
	path, err := a.ImageLocation(id, reference)
	if err != nil {
		return nil, err
	}
	data, err := readCopyImage(path)
	if err != nil {
		return nil, err
	}
	return &imageCopySource{LocalPath: path, Name: filepath.Base(path), Data: data}, nil
}

func (a *App) moveImageTo(id, reference, target string, overwrite bool) (*ImageCopyResult, error) {
	source, err := a.localImageMoveSource(id, reference)
	if err != nil {
		return nil, err
	}
	return a.copyImageSourceTo(id, source, target, overwrite, true)
}

func (a *App) MoveImage(id, reference string) (*ImageCopyResult, error) {
	source, err := a.localImageMoveSource(id, reference)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	d, ok := a.docs[id]
	base := ""
	if ok {
		if d.Path != "" {
			base = filepath.Dir(d.Path)
		}
		base, _, err = documentImageRoot(d.Content, base)
	}
	a.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("文档不存在")
	}
	if err != nil {
		return nil, err
	}
	if base == "" {
		base = filepath.Dir(source.LocalPath)
	}
	target, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "移动并更新图片路径到…", DefaultDirectory: base, DefaultFilename: source.Name, Filters: []runtime.FileFilter{{DisplayName: "All Files", Pattern: "*.*"}}})
	if err != nil || target == "" {
		return nil, err
	}
	return a.copyImageSourceTo(id, source, target, true, true)
}
