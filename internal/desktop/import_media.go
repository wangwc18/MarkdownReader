package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *App) importRoot(id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", errors.New("导入资源标识无效")
	}
	return filepath.Join(a.dataDir, "imports", id), nil
}
func importAssetPrefix(id string) string { return filepath.Join("assets", "import-"+id) }

func (a *App) ImportPath(path string) (*Document, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("请选择文档文件")
	}
	id := uuid.NewString()
	root, _ := a.importRoot(id)
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.pandoc(), path, "--resource-path="+filepath.Dir(path), "--extract-media="+filepath.ToSlash(importAssetPrefix(id)), "--wrap=none", "-t", "markdown+tex_math_dollars+yaml_metadata_block+footnotes-link_attributes")
	hideConversionConsole(cmd)
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("导入失败：%s", exit.Stderr)
		}
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.newDoc()
	d.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + ".md"
	d.Content = strings.ReplaceAll(string(b), "\r\n", "\n")
	d.Dirty = true
	d.ImportID = id
	d.BaseURL = a.registerRoot(root)
	if err = a.writeRecovery(); err != nil {
		delete(a.docs, d.ID)
		return nil, err
	}
	copy := *d
	return &copy, nil
}

// The extracted references already use this unique relative prefix, so saving
// copies assets without rewriting Markdown or touching the source document.
func (a *App) copyImportedMedia(id, destination string) error {
	root, err := a.importRoot(id)
	if err != nil {
		return err
	}
	prefix := importAssetPrefix(id)
	source := filepath.Join(root, prefix)
	if _, err = os.Stat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("导入资源不能包含符号链接")
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel != "." && !filepath.IsLocal(rel) {
			return errors.New("导入资源路径越界")
		}
		parent := filepath.Join(prefix, rel)
		if !entry.IsDir() {
			parent = filepath.Dir(parent)
		}
		folder, err := imageStorageFolder(destination, parent)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 64*1024*1024 {
			return errors.New("导入资源无效或单文件超过64MB")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(folder, entry.Name())
		if existing, readErr := os.ReadFile(target); readErr == nil {
			if bytes.Equal(existing, b) {
				return nil
			}
			return fmt.Errorf("导入资源与已有文件冲突：%s", target)
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
		if err = createRestoredFile(target, b, 0644); err != nil {
			if existing, readErr := os.ReadFile(target); readErr != nil || !bytes.Equal(existing, b) {
				return err
			}
		}
		return nil
	})
}
