package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// PrepareImageReference is used when a new image URL is submitted in the image
// address dialog. It does not rewrite document content or existing references.
func (a *App) PrepareImageReference(id, reference, label string) (string, error) {
	u, err := url.Parse(reference)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return reference, nil
	}
	a.mu.Lock()
	d := a.docs[id]
	if d == nil {
		a.mu.Unlock()
		return "", fmt.Errorf("文档不存在")
	}
	snapshot, settings := *d, a.settings
	a.mu.Unlock()
	action, err := imageInsertionActionFor(snapshot.Content, settings, true)
	if err != nil {
		return "", err
	}
	if action == "none" {
		return reference, nil
	}
	if snapshot.Path == "" {
		return "", fmt.Errorf("请先保存文档，再应用网络图片插入规则")
	}
	source, err := downloadHTTPImage(reference, label)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(a.dataDir, "image-network-insert-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dir)
	path := filepath.Join(dir, source.Name)
	if err = os.WriteFile(path, source.Data, 0600); err != nil {
		return "", err
	}
	defer os.Remove(path)
	// Downloads may take a while. Do not invoke an uploader or write a document
	// asset if the original document changed during that download.
	a.mu.Lock()
	current := a.docs[id]
	unchanged := current != nil && current.Content == snapshot.Content && current.Path == snapshot.Path
	if !unchanged {
		a.mu.Unlock()
		return "", fmt.Errorf("文档已变化，未应用网络图片插入规则")
	}
	if action == "upload" {
		a.mu.Unlock()
		result, err := a.uploadInsertedImage(snapshot, settings, path)
		if err != nil {
			return "", err
		}
		if err = validateUploadedTemporaryReference(snapshot, a.base, result, dir); err != nil {
			return "", err
		}
		return result, nil
	}
	defer a.mu.Unlock()
	base := filepath.Dir(snapshot.Path)
	root, _, err := documentImageRoot(snapshot.Content, base)
	if err != nil {
		return "", err
	}
	folder, _, err := insertedImageFolder(snapshot.Content, base, settings.ImageFolder, snapshot.Path, settings.ImageFolderCustom)
	if err != nil {
		return "", err
	}
	target, err := copyInsertedImage(path, folder)
	if err != nil {
		return "", err
	}
	result, err := imageReference(root, target, settings.RelativeImages)
	if err == nil && u.Fragment != "" {
		result += "#" + u.EscapedFragment()
	}
	return result, err
}
