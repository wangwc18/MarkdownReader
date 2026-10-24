package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// A converter stage lives in the canonical system temporary directory, never
// next to a document or destination. Handles bind operations to that directory.
type exportStage struct {
	Path                     string
	parentPath, leaf         string
	root, parent             *os.Root
	identity, parentIdentity os.FileInfo
}

func reparseFile(info os.FileInfo) bool {
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return true
	}
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attributes.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func newExportStage(forbiddenRoots ...string) (*exportStage, error) {
	temp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return nil, err
	}
	temp, err = filepath.Abs(temp)
	if err != nil {
		return nil, err
	}
	for _, forbidden := range forbiddenRoots {
		if forbidden == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(forbidden)
		if err != nil {
			return nil, err
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(resolved, temp)
		if err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			return nil, fmt.Errorf("系统临时目录位于工作目录内，请为文件夹导出指定独立的临时目录")
		}
	}
	parent, err := os.OpenRoot(temp)
	if err != nil {
		return nil, err
	}
	parentInfo, err := parent.Stat(".")
	if err != nil {
		parent.Close()
		return nil, err
	}
	path, err := os.MkdirTemp(temp, "MarkdownReader-export-stage-")
	if err != nil {
		parent.Close()
		return nil, err
	}
	stage := &exportStage{Path: path, parentPath: temp, leaf: filepath.Base(path), parent: parent, parentIdentity: parentInfo}
	stage.root, err = parent.OpenRoot(stage.leaf)
	if err != nil {
		parent.Remove(stage.leaf)
		parent.Close()
		return nil, err
	}
	stage.identity, err = stage.root.Stat(".")
	if err == nil {
		err = stage.verifyIdentity()
	}
	if err != nil {
		stage.Close()
		return nil, err
	}
	return stage, nil
}

func (s *exportStage) Close() {
	if s.root != nil {
		s.root.Close()
		s.root = nil
	}
	if s.parent != nil {
		s.parent.Close()
		s.parent = nil
	}
}

func (s *exportStage) verifyIdentity() error {
	if s.root == nil || s.parent == nil {
		return fmt.Errorf("导出暂存目录句柄已关闭")
	}
	if !filepath.IsAbs(s.Path) || filepath.Dir(s.Path) != s.parentPath || filepath.Base(s.Path) != s.leaf || !strings.HasPrefix(s.leaf, "MarkdownReader-export-stage-") {
		return fmt.Errorf("导出暂存路径不在预期临时目录内")
	}
	parent, err := os.Lstat(s.parentPath)
	if err != nil {
		return err
	}
	if reparseFile(parent) || !os.SameFile(parent, s.parentIdentity) {
		return fmt.Errorf("导出暂存父目录已变化")
	}
	info, err := s.parent.Lstat(s.leaf)
	if err != nil {
		return err
	}
	if reparseFile(info) || !info.IsDir() || !os.SameFile(info, s.identity) {
		return fmt.Errorf("导出暂存根目录已变化或成为链接")
	}
	return nil
}

func (s *exportStage) entries() ([]string, []string, error) {
	if err := s.verifyIdentity(); err != nil {
		return nil, nil, err
	}
	var files, dirs []string
	err := fs.WalkDir(s.root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := s.root.Lstat(name)
		if err != nil {
			return err
		}
		if reparseFile(info) {
			return fmt.Errorf("输出包含链接或目录联接：%s", name)
		}
		if info.IsDir() {
			if name != "." {
				dirs = append(dirs, name)
			}
		} else if info.Mode().IsRegular() {
			files = append(files, name)
		} else {
			return fmt.Errorf("输出包含非普通文件：%s", name)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if err = s.verifyIdentity(); err != nil {
		return nil, nil, err
	}
	return files, dirs, nil
}

// Call immediately before consuming converter output, and again before cleanup.
func (s *exportStage) Verify() error {
	_, _, err := s.entries()
	if err != nil {
		return fmt.Errorf("%w；暂存内容已保留：%s", err, s.Path)
	}
	return nil
}

// Never use RemoveAll on converter-controlled paths. Root.Remove unlinks only
// the named entry; parent links cannot escape the bound root even during races.
func (s *exportStage) Cleanup() error {
	files, dirs, err := s.entries()
	if err != nil {
		return fmt.Errorf("%w；未清理暂存目录：%s", err, s.Path)
	}
	remove := func(name string) error {
		if err := s.verifyIdentity(); err != nil {
			return err
		}
		info, err := s.root.Lstat(name)
		if err != nil {
			return err
		}
		if reparseFile(info) {
			return fmt.Errorf("清理前发现链接，停止清理：%s", name)
		}
		return s.root.Remove(name)
	}
	for _, file := range files {
		if err = remove(file); err != nil {
			return fmt.Errorf("%w；剩余暂存内容已保留：%s", err, s.Path)
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err = remove(dirs[i]); err != nil {
			return fmt.Errorf("%w；剩余暂存内容已保留：%s", err, s.Path)
		}
	}
	if err = s.verifyIdentity(); err != nil {
		return err
	}
	if err = s.parent.Remove(s.leaf); err != nil {
		return fmt.Errorf("暂存目录未清理：%s：%w", s.Path, err)
	}
	s.Close()
	return nil
}
