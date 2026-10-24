package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const clipboardImageLimit = 20 * 1024 * 1024

func imageStorageFolder(root, name string, custom ...bool) (string, error) {
	if name == "" {
		name = "assets"
	}
	if len(custom) > 0 && custom[0] {
		if !filepath.IsAbs(name) && (filepath.VolumeName(name) != "" || strings.HasPrefix(name, `\`)) {
			return "", errors.New("图片目录须使用完整绝对路径或相对路径")
		}
		folder := filepath.Clean(name)
		if !filepath.IsAbs(folder) {
			folder = filepath.Join(root, folder)
		}
		if err := os.MkdirAll(folder, 0755); err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(folder)
	}
	if !filepath.IsLocal(name) {
		return "", errors.New("图片目录必须位于文档目录内")
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	folder := root
	for _, part := range strings.Split(filepath.Clean(name), string(os.PathSeparator)) {
		if part == "." {
			continue
		}
		folder = filepath.Join(folder, part)
		if err = os.Mkdir(folder, 0755); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, err := os.Lstat(folder)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("图片目录不能包含符号链接或非目录项")
		}
	}
	return folder, nil
}

func (a *App) ImportClipboardImage(id, encoded string) (string, error) {
	if len(encoded) > base64.StdEncoding.EncodedLen(clipboardImageLimit) {
		return "", errors.New("剪贴板图片不能超过20MB")
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("剪贴板图片数据无效")
	}
	if len(b) > clipboardImageLimit {
		return "", errors.New("剪贴板图片不能超过20MB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16*1024*1024 {
		return "", errors.New("图片无效或超过1600万像素")
	}
	extension, ok := map[string]string{"png": ".png", "jpeg": ".jpg", "gif": ".gif", "webp": ".webp", "bmp": ".bmp"}[format]
	if !ok {
		return "", errors.New("不支持的剪贴板图片格式")
	}
	if _, _, err = image.Decode(bytes.NewReader(b)); err != nil {
		return "", errors.New("剪贴板图片数据不完整")
	}
	a.mu.Lock()
	locked := true
	defer func() {
		if locked {
			a.mu.Unlock()
		}
	}()
	d, ok := a.docs[id]
	if !ok {
		return "", errors.New("文档不存在")
	}
	if d.Path == "" {
		return "", errors.New("请先保存文档，再粘贴截图")
	}
	action, err := imageInsertionAction(d.Content, a.settings)
	if err != nil {
		return "", err
	}
	if action == "upload" {
		snapshot, settings := *d, a.settings
		a.mu.Unlock()
		locked = false
		dir, err := os.MkdirTemp(a.dataDir, "image-insert-upload-*")
		if err != nil {
			return "", err
		}
		defer os.Remove(dir)
		path := filepath.Join(dir, "clipboard"+extension)
		if err = os.WriteFile(path, b, 0600); err != nil {
			return "", err
		}
		defer os.Remove(path)
		reference, err := a.uploadInsertedImage(snapshot, settings, path)
		if err != nil {
			return "", err
		}
		if err = validateUploadedTemporaryReference(snapshot, a.base, reference, dir); err != nil {
			return "", err
		}
		return reference, nil
	}
	root := filepath.Dir(d.Path)
	referenceRoot, _, err := documentImageRoot(d.Content, root)
	if err != nil {
		return "", err
	}
	folder, _, err := insertedImageFolder(d.Content, root, a.settings.ImageFolder, d.Path, a.settings.ImageFolderCustom)
	if err != nil {
		return "", err
	}
	path := filepath.Join(folder, "clipboard-"+digest(b)[:20]+extension)
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if !bytes.Equal(existing, b) {
			return "", errors.New("图片文件名冲突，已有文件保持不变")
		}
	} else if !os.IsNotExist(readErr) {
		return "", readErr
	} else if err = createRestoredFile(path, b, 0644); err != nil {
		// Another window may have atomically stored the same clipboard image.
		if existing, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(existing, b) {
			return "", err
		}
	}
	return imageReference(referenceRoot, path, a.settings.RelativeImages)
}
