package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ImageCopiesResult struct {
	Original   string            `json:"original"`
	References map[string]string `json:"references"`
	Token      string            `json:"token"`
	Copied     int               `json:"copied"`
	Skipped    int               `json:"skipped"`
}

func imageReferenceForTarget(d Document, target string, relative bool) (string, error) {
	base := ""
	if d.Path != "" {
		base = filepath.Dir(d.Path)
	}
	root, configured, err := documentImageRoot(d.Content, base)
	if err != nil {
		return "", err
	}
	if root != "" && (relative || configured) {
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, target)
		if err != nil {
			return "", err
		}
		ref := copiedImagePath(rel)
		if configured {
			ref = "/" + ref
		}
		return ref, nil
	}
	if root == "" {
		return copiedImagePath(target), nil
	}
	return "file:///" + copiedImagePath(target), nil
}

func copiedImagePath(path string) string {
	var result strings.Builder
	for _, r := range filepath.ToSlash(path) {
		if r < 33 || strings.ContainsRune("%#?<>\"`[]", r) {
			result.WriteString(url.PathEscape(string(r)))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func (a *App) copyImagesTo(id string, references []string, directory string, labelSets ...map[string]string) (*ImageCopiesResult, error) {
	var labels map[string]string
	if len(labelSets) > 0 {
		labels = labelSets[0]
	}
	return a.transferImagesTo(id, references, directory, labels, false)
}

func (a *App) moveImagesTo(id string, references []string, directory string) (*ImageCopiesResult, error) {
	return a.transferImagesTo(id, references, directory, nil, true)
}

func (a *App) transferImagesTo(id string, references []string, directory string, labels map[string]string, moving bool) (*ImageCopiesResult, error) {
	if len(references) > 2000 {
		return nil, fmt.Errorf("单批图片数量超过2000")
	}
	directory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("目标必须是已有目录")
	}
	a.mu.Lock()
	d, ok := a.docs[id]
	if !ok {
		a.mu.Unlock()
		return nil, fmt.Errorf("文档不存在")
	}
	snapshot := *d
	relative := a.settings.RelativeImages
	a.mu.Unlock()
	result := &ImageCopiesResult{Original: snapshot.Content, References: map[string]string{}}
	op := &imageCopyOperation{DocumentID: id, Applied: false}
	removals := []*imageCopyOperation{}
	bySource, reserved := map[string]string{}, map[string]bool{}
	plannedBytes := 0
	remoteSources := map[string]*imageCopySource{}
	canonicalSources := map[string]*imageCopySource{}
	for _, reference := range references {
		if _, seen := result.References[reference]; seen {
			continue
		}
		u, parseErr := url.Parse(reference)
		remote := parseErr == nil && (u.Scheme == "http" || u.Scheme == "https")
		embedded := len(reference) >= 5 && strings.EqualFold(reference[:5], "data:")
		if moving && (remote || embedded) {
			result.Skipped++
			continue
		}
		if parseErr == nil && u.Scheme == "blob" {
			result.Skipped++
			continue
		}
		var source *imageCopySource
		var err error
		remoteKey := ""
		if remote {
			copyURL := *u
			copyURL.Fragment = ""
			remoteKey = copyURL.String()
			source = remoteSources[remoteKey]
		} else if embedded {
			remoteKey, _, _ = strings.Cut(reference, "#")
			source = remoteSources[remoteKey]
		}
		if source == nil {
			source, err = a.imageSourceForCopy(id, reference, labels[reference])
			if err != nil {
				return nil, err
			}
			if remote || embedded {
				if existing := canonicalSources[source.Key]; existing != nil {
					source = existing
				} else {
					canonicalSources[source.Key] = source
				}
				remoteSources[remoteKey] = source
			}
		}
		key := source.Key
		if moving && source.LocalPath == "" {
			result.Skipped++
			continue
		}
		ref, seen := bySource[key]
		if !seen {
			after := source.Data
			plannedBytes += len(after)
			if plannedBytes > 128*1024*1024 {
				return nil, fmt.Errorf("单批图片超过128MB")
			}
			name := source.Name
			target := filepath.Join(directory, name)
			for serial := 0; ; serial++ {
				_, statErr := os.Lstat(target)
				if source.LocalPath != "" && strings.EqualFold(target, source.LocalPath) {
					break
				}
				if os.IsNotExist(statErr) && !reserved[strings.ToLower(target)] {
					break
				}
				if statErr != nil && !os.IsNotExist(statErr) {
					return nil, statErr
				}
				ext := filepath.Ext(name)
				target = filepath.Join(directory, fmt.Sprintf("%s-%d%d%s", strings.TrimSuffix(name, ext), time.Now().UnixMilli(), serial, ext))
			}
			ref, err = imageReferenceForTarget(snapshot, target, relative)
			if err != nil {
				return nil, err
			}
			if source.LocalPath == "" || !strings.EqualFold(source.LocalPath, target) {
				op.Children = append(op.Children, &imageCopyOperation{DocumentID: id, Target: target, After: after})
				if moving {
					removals = append(removals, &imageCopyOperation{DocumentID: id, Target: source.LocalPath, Before: after, Existed: true, RemoveAfter: true})
				}
				result.Copied++
			}
			reserved[strings.ToLower(target)] = true
			bySource[key] = ref
		}
		if remote || embedded {
			if _, fragment, ok := strings.Cut(reference, "#"); ok && fragment != "" {
				decoded, err := url.PathUnescape(fragment)
				if err != nil {
					return nil, err
				}
				ref += "#" + (&url.URL{Fragment: decoded}).EscapedFragment()
			}
		} else if suffix := strings.IndexAny(reference, "?#"); suffix >= 0 {
			ref += reference[suffix:]
		}
		result.References[reference] = ref
	}
	op.Children = append(op.Children, removals...)
	if len(op.Children) == 0 {
		return result, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.docs[id] == nil || a.docs[id].Content != snapshot.Content || a.docs[id].Path != snapshot.Path || a.docs[id].BaseURL != snapshot.BaseURL {
		return nil, fmt.Errorf("文档已变化，请重新复制图片")
	}
	used := imageCopyBytes(op)
	for _, existing := range a.imageCopies {
		used += imageCopyBytes(existing)
	}
	if used > 128*1024*1024 || len(a.imageCopies) >= 2000 {
		return nil, fmt.Errorf("图片撤销记录过多，请关闭不再编辑的文档后重试")
	}
	if err := changeImageCopyState(op, true); err != nil {
		return nil, err
	}
	if a.imageCopies == nil {
		a.imageCopies = map[string]*imageCopyOperation{}
	}
	result.Token = uuid.NewString()
	a.imageCopies[result.Token] = op
	return result, nil
}

func (a *App) MoveImages(id string, references []string) (*ImageCopiesResult, error) {
	if len(references) == 0 {
		return nil, fmt.Errorf("文档中没有图片")
	}
	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择图片移动目标目录"})
	if err != nil || directory == "" {
		return nil, err
	}
	return a.moveImagesTo(id, references, directory)
}

func (a *App) CopyImages(id string, references []string, labels map[string]string) (*ImageCopiesResult, error) {
	if len(references) == 0 {
		return nil, fmt.Errorf("文档中没有图片")
	}
	a.traceLifecycle("copy-all-dialog-open", nil)
	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择图片存储路径"})
	a.traceLifecycle("copy-all-dialog-return", map[string]any{"selected": directory != "", "error": err != nil})
	if err != nil || directory == "" {
		return nil, err
	}
	return a.copyImagesTo(id, references, directory, labels)
}
