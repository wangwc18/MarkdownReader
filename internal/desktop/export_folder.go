package desktop

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type folderExportChange struct {
	name, backup string
	existed      bool
	hash         [32]byte
}

// Check every target component without following converter-created links.
func folderExportTarget(root *os.Root, name string, directory bool) error {
	parts := strings.Split(filepath.Clean(name), string(os.PathSeparator))
	for i := range parts {
		part := filepath.Join(parts[:i+1]...)
		info, err := root.Lstat(part)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if reparseFile(info) {
			return fmt.Errorf("导出目标包含链接：%s", part)
		}
		wantDir := directory || i < len(parts)-1
		if wantDir != info.IsDir() || !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("导出目标的文件和目录类型冲突：%s", part)
		}
	}
	return nil
}

// Merge only the files produced by the converter. Unmentioned files stay in
// place. Backups remain beside the destination until all replacements succeed.
func mergeExportFolder(stage *exportStage, destination string) (resultErr error) {
	files, dirs, err := stage.entries()
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	identity, err := os.Lstat(destination)
	if err != nil {
		return err
	}
	if reparseFile(identity) || !identity.IsDir() {
		return fmt.Errorf("请选择普通输出文件夹")
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	bound, err := root.Stat(".")
	if err != nil || !os.SameFile(identity, bound) {
		return fmt.Errorf("导出目标目录已变化")
	}
	verify := func() error {
		current, err := os.Lstat(destination)
		if err != nil {
			return err
		}
		if reparseFile(current) || !os.SameFile(identity, current) {
			return fmt.Errorf("导出目标目录已变化")
		}
		return nil
	}
	for _, dir := range dirs {
		if err := folderExportTarget(root, dir, true); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := folderExportTarget(root, file, false); err != nil {
			return err
		}
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	transaction := fmt.Sprintf(".markdownreader-export-%x", token)
	if err := root.Mkdir(transaction, 0700); err != nil {
		return err
	}
	transactionRoot, err := root.OpenRoot(transaction)
	if err != nil {
		return err
	}
	defer transactionRoot.Close()
	transactionIdentity, err := transactionRoot.Stat(".")
	if err != nil {
		return err
	}
	verifyDestination := verify
	verify = func() error {
		if err := verifyDestination(); err != nil {
			return err
		}
		info, err := root.Lstat(transaction)
		if err != nil {
			return err
		}
		if reparseFile(info) || !os.SameFile(info, transactionIdentity) {
			return fmt.Errorf("导出恢复目录已变化：%s", filepath.Join(destination, transaction))
		}
		return nil
	}
	var temporary []string
	var changes []folderExportChange
	var createdDirs []string
	defer func() {
		if resultErr != nil {
			// If another process changed a committed file, preserve the backup
			// instead of overwriting its newer work during rollback.
			for i := len(changes) - 1; i >= 0; i-- {
				change := changes[i]
				rollbackErr := verify()
				if rollbackErr == nil {
					rollbackErr = folderExportTarget(root, change.name, false)
				}
				if rollbackErr == nil {
					current, err := root.ReadFile(change.name)
					if err != nil {
						rollbackErr = err
					} else if sha256.Sum256(current) != change.hash {
						rollbackErr = fmt.Errorf("文件在回滚前已变化：%s", change.name)
					}
				}
				if rollbackErr == nil {
					if change.existed {
						rollbackErr = root.Rename(change.backup, change.name)
					} else {
						rollbackErr = root.Remove(change.name)
					}
				}
				if rollbackErr != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("回滚停止：%w；恢复文件保留于 %s", rollbackErr, filepath.Join(destination, transaction)))
					return
				}
			}
			for i := len(createdDirs) - 1; i >= 0; i-- {
				if err := verify(); err != nil {
					resultErr = errors.Join(resultErr, err)
					return
				}
				// Remove only an empty directory; never recursively remove it.
				if err := root.Remove(createdDirs[i]); err != nil {
					resultErr = errors.Join(resultErr, err)
					return
				}
			}
		}
		if err := verify(); err != nil {
			resultErr = errors.Join(resultErr, err)
			return
		}
		for _, name := range temporary {
			if err := verify(); err != nil {
				resultErr = errors.Join(resultErr, err)
				return
			}
			if err := transactionRoot.Remove(filepath.Base(name)); err != nil && !os.IsNotExist(err) {
				resultErr = errors.Join(resultErr, fmt.Errorf("恢复文件保留于 %s：%w", filepath.Join(destination, transaction), err))
				return
			}
		}
		if err := root.Remove(transaction); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	manifest := filepath.Join(transaction, "恢复清单.json")
	temporary = append(temporary, manifest)
	for _, dir := range dirs {
		if err := verify(); err != nil {
			return err
		}
		if err := folderExportTarget(root, dir, true); err != nil {
			return err
		}
		if _, err := root.Lstat(dir); os.IsNotExist(err) {
			if err := root.Mkdir(dir, 0700); err != nil {
				return err
			}
			createdDirs = append(createdDirs, dir)
		} else if err != nil {
			return err
		}
	}
	for index, file := range files {
		if err := verify(); err != nil {
			return err
		}
		if err := stage.verifyIdentity(); err != nil {
			return err
		}
		info, err := stage.root.Lstat(file)
		if err != nil {
			return err
		}
		if reparseFile(info) || !info.Mode().IsRegular() {
			return fmt.Errorf("导出输出文件已变化：%s", file)
		}
		if err := folderExportTarget(root, file, false); err != nil {
			return err
		}
		data, err := stage.root.ReadFile(file)
		if err != nil {
			return err
		}
		change := folderExportChange{name: file, hash: sha256.Sum256(data)}
		var originalHash [32]byte
		if old, err := root.ReadFile(file); err == nil {
			change.existed = true
			originalHash = sha256.Sum256(old)
			change.backup = filepath.Join(transaction, fmt.Sprintf("backup-%d", index))
			if err := transactionRoot.WriteFile(filepath.Base(change.backup), old, 0600); err != nil {
				return err
			}
			temporary = append(temporary, change.backup)
			// A file edited during backup must not be silently replaced.
			current, err := root.ReadFile(file)
			if err != nil || !bytes.Equal(old, current) {
				return fmt.Errorf("备份期间目标文件已变化：%s", file)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		pending := filepath.Join(transaction, fmt.Sprintf("new-%d", index))
		if err := transactionRoot.WriteFile(filepath.Base(pending), data, 0600); err != nil {
			return err
		}
		temporary = append(temporary, pending)
		if err := verify(); err != nil {
			return err
		}
		if err := folderExportTarget(root, file, false); err != nil {
			return err
		}
		var recovery []map[string]interface{}
		for _, item := range append(append([]folderExportChange{}, changes...), change) {
			recovery = append(recovery, map[string]interface{}{"file": item.name, "backup": item.backup, "previouslyExisted": item.existed})
		}
		manifestBytes, err := json.MarshalIndent(map[string]interface{}{"destination": destination, "files": recovery}, "", "  ")
		if err != nil {
			return err
		}
		if err := transactionRoot.WriteFile(filepath.Base(manifest), manifestBytes, 0600); err != nil {
			return err
		}
		current, err := root.ReadFile(file)
		if change.existed {
			if err != nil || sha256.Sum256(current) != originalHash {
				return fmt.Errorf("提交前目标文件已变化：%s", file)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("提交前出现新目标文件：%s", file)
		}
		if err := root.Rename(pending, file); err != nil {
			return err
		}
		changes = append(changes, change)
	}
	return nil
}

func (a *App) programFolder(doc Document, preset ExportPreset, destination string) (message string, resultErr error) {
	forbidden := []string{a.base, destination}
	if doc.Path != "" {
		forbidden = append(forbidden, filepath.Dir(doc.Path))
	}
	stage, err := newExportStage(forbidden...)
	if err != nil {
		return "", err
	}
	defer stage.Close()
	defer func() { resultErr = errors.Join(resultErr, stage.Cleanup()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	message, err = a.runExportProgram(ctx, doc, preset, stage.Path, destination)
	if err != nil {
		return message, err
	}
	if err := stage.Verify(); err != nil {
		return message, err
	}
	return message, mergeExportFolder(stage, destination)
}
