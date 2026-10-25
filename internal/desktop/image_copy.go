package desktop

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type imageCopyOperation struct {
	DocumentID, Target string
	Before, After      []byte
	Existed, Applied   bool
	RemoveAfter        bool
	Children           []*imageCopyOperation
}
type ImageCopyResult struct {
	Original  string `json:"original"`
	Reference string `json:"reference"`
	Token     string `json:"token"`
}

func readCopyImage(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 20*1024*1024 {
		return nil, fmt.Errorf("图片必须是20MB以内的普通文件")
	}
	b, err := io.ReadAll(io.LimitReader(f, 20*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 20*1024*1024 {
		return nil, fmt.Errorf("图片不能超过20MB")
	}
	return b, nil
}

func (a *App) copyImageTo(id, reference, target string, overwrite bool, labels ...string) (*ImageCopyResult, error) {
	label := ""
	if len(labels) > 0 {
		label = labels[0]
	}
	source, err := a.imageSourceForCopy(id, reference, label)
	if err != nil {
		return nil, err
	}
	return a.copyImageSourceTo(id, source, target, overwrite)
}

func (a *App) copyImageSourceTo(id string, source *imageCopySource, target string, overwrite bool, moving ...bool) (*ImageCopyResult, error) {
	var err error
	after := source.Data
	move := len(moving) > 0 && moving[0]
	var lockedSource *os.File
	if move {
		if source.LocalPath == "" {
			return nil, fmt.Errorf("当前移动操作需要本地图片")
		}
		lockedSource, err = openImageSnapshotForRemoval(source.LocalPath, after)
		if err != nil {
			return nil, err
		}
		defer lockedSource.Close()
	}
	if !filepath.IsAbs(target) {
		return nil, fmt.Errorf("目标图片路径无效")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	target = filepath.Join(parent, filepath.Base(target))
	var before []byte
	existed := false
	sameFile := false
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("目标不能是目录或符号链接")
		}
		existed = true
		if move {
			sourceInfo, statErr := lockedSource.Stat()
			if statErr != nil {
				return nil, statErr
			}
			sameFile = os.SameFile(sourceInfo, info)
		}
		if sameFile {
			before = after
		} else {
			before, err = readCopyImage(target)
		}
		if err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return nil, fmt.Errorf("文档不存在")
	}
	result := &ImageCopyResult{Original: d.Content}
	// Generate the reference before modifying a destination. imageReference
	// needs an existing target, so calculate its canonical relative URI here.
	result.Reference, err = imageReferenceForTarget(*d, target, a.settings.RelativeImages)
	if err != nil {
		return nil, err
	}
	unchanged := existed && bytes.Equal(before, after)
	if sameFile {
		return result, nil
	}
	if unchanged && !move {
		return result, nil
	}
	if existed && !unchanged && !overwrite {
		return nil, fmt.Errorf("目标文件已存在，尚未确认覆盖")
	}
	used := len(before) + len(after)
	if unchanged {
		used = 0
	}
	if move {
		used += len(after)
	}
	for _, op := range a.imageCopies {
		used += imageCopyBytes(op)
	}
	if used > 128*1024*1024 {
		return nil, fmt.Errorf("图片撤销记录超过128MB，请关闭不再编辑的文档后重试")
	}
	if len(a.imageCopies) >= 2000 {
		return nil, fmt.Errorf("图片撤销记录过多，请关闭不再编辑的文档后重试")
	}
	if unchanged {
		err = nil
	} else if existed {
		err = atomicWrite(target, after)
	} else {
		err = createRestoredFile(target, after, 0644)
	}
	if err != nil {
		return nil, err
	}
	operation := &imageCopyOperation{DocumentID: id, Target: target, Before: before, After: after, Existed: existed, Applied: true}
	if move {
		removal := &imageCopyOperation{DocumentID: id, Target: source.LocalPath, Before: after, Existed: true, RemoveAfter: true, Applied: true}
		if err = markImageSnapshotForRemoval(lockedSource); err != nil {
			if !unchanged {
				if rollback := changeImageCopyState(operation, false); rollback != nil {
					return nil, fmt.Errorf("源图移除失败：%v；目标回滚失败：%w", err, rollback)
				}
			}
			return nil, err
		}
		children := []*imageCopyOperation{removal}
		if !unchanged {
			children = []*imageCopyOperation{operation, removal}
		}
		operation = &imageCopyOperation{DocumentID: id, Children: children, Applied: true}
	}
	if a.imageCopies == nil {
		a.imageCopies = map[string]*imageCopyOperation{}
	}
	token := uuid.NewString()
	a.imageCopies[token] = operation
	result.Token = token
	return result, nil
}

func (a *App) CopyImage(id, reference, label string) (*ImageCopyResult, error) {
	source, err := a.imageSourceForCopy(id, reference, label)
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
		if source.LocalPath != "" {
			base = filepath.Dir(source.LocalPath)
		}
	}
	target, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "复制并更新图片路径到…", DefaultDirectory: base, DefaultFilename: source.Name, Filters: []runtime.FileFilter{{DisplayName: "All Files", Pattern: "*.*"}}})
	if err != nil || target == "" {
		return nil, err
	}
	return a.copyImageSourceTo(id, source, target, true)
}

func (a *App) SetImageCopyState(id, token string, applied bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	op := a.imageCopies[token]
	if op == nil || op.DocumentID != id || a.docs[id] == nil {
		return fmt.Errorf("图片操作历史不可用")
	}
	if op.Applied == applied {
		return nil
	}
	return changeImageCopyState(op, applied)
}

func imageCopyBytes(op *imageCopyOperation) int {
	n := len(op.Before) + len(op.After)
	for _, child := range op.Children {
		n += imageCopyBytes(child)
	}
	return n
}

func checkImageCopyState(op *imageCopyOperation) error {
	for _, child := range op.Children {
		if err := checkImageCopyState(child); err != nil {
			return err
		}
	}
	if len(op.Children) > 0 {
		return nil
	}
	expected, exists := op.Before, op.Existed
	if op.Applied {
		expected, exists = op.After, !op.RemoveAfter
	}
	info, err := os.Lstat(op.Target)
	if exists {
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("图片副本已被外部修改或删除，无法切换历史")
		}
		current, err := readCopyImage(op.Target)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, expected) {
			return fmt.Errorf("图片副本已被外部修改，无法切换历史")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("目标位置已有文件，无法重做图片复制")
	}
	return nil
}

func changeImageCopyState(op *imageCopyOperation, applied bool) error {
	if err := checkImageCopyState(op); err != nil {
		return err
	}
	return writeImageCopyState(op, applied)
}

func writeImageCopyState(op *imageCopyOperation, applied bool) error {
	if len(op.Children) > 0 {
		completed := []*imageCopyOperation{}
		children := append([]*imageCopyOperation(nil), op.Children...)
		if !applied {
			for left, right := 0, len(children)-1; left < right; left, right = left+1, right-1 {
				children[left], children[right] = children[right], children[left]
			}
		}
		for _, child := range children {
			if err := writeImageCopyState(child, applied); err != nil {
				for i := len(completed) - 1; i >= 0; i-- {
					if rollback := changeImageCopyState(completed[i], !applied); rollback != nil {
						return fmt.Errorf("图片操作失败：%v；回滚失败：%w", err, rollback)
					}
				}
				return err
			}
			completed = append(completed, child)
		}
		op.Applied = applied
		return nil
	}
	var err error
	if applied && op.RemoveAfter {
		err = removeImageSnapshot(op.Target, op.Before)
	} else if !applied && op.RemoveAfter {
		err = createRestoredFile(op.Target, op.Before, 0644)
	} else if applied {
		if op.Existed {
			err = atomicWrite(op.Target, op.After)
		} else {
			err = createRestoredFile(op.Target, op.After, 0644)
		}
	} else if op.Existed {
		err = atomicWrite(op.Target, op.Before)
	} else {
		err = os.Remove(op.Target)
	}
	if err != nil {
		return err
	}
	op.Applied = applied
	return nil
}
