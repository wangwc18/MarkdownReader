package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.yaml.in/yaml/v3"
)

// Image roots belong to a document's front matter, never to global preferences.
func documentImageRoot(content, documentRoot string) (string, bool, error) {
	metadata, err := exportMetadata(content)
	if err != nil {
		return "", false, err
	}
	if metadata == nil {
		return documentRoot, false, nil
	}
	value, err := metadataLookup(metadata, []string{"typora-root-url"}, 0)
	if err != nil {
		return "", false, err
	}
	if value == nil || value.Tag == "!!null" {
		return documentRoot, false, nil
	}
	if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
		return "", false, fmt.Errorf("typora-root-url 必须是目录字符串")
	}
	if value.Value == "" {
		return documentRoot, false, nil
	}
	root := filepath.FromSlash(value.Value)
	if !filepath.IsAbs(root) {
		if documentRoot == "" {
			return "", false, fmt.Errorf("相对图片根目录需要先保存文档")
		}
		root = filepath.Join(documentRoot, root)
	}
	return filepath.Clean(root), true, nil
}

func withDocumentImageRoot(content, directory string) (string, error) {
	return withDocumentImageSetting(content, "typora-root-url", directory)
}

func withDocumentImageSetting(content, setting, directory string) (string, error) {
	metadata, err := exportMetadata(content)
	if err != nil {
		return "", err
	}
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	bom := ""
	if strings.HasPrefix(content, "\ufeff") {
		bom = "\ufeff"
		content = strings.TrimPrefix(content, bom)
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	rawLines := strings.SplitAfter(content, "\n")
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: directory}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	entry := setting + ": " + strings.TrimSuffix(string(encoded), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != "---" {
		return bom + "---" + eol + entry + eol + "---" + eol + eol + content, nil
	}
	end := 1
	for end < len(lines) && strings.TrimRight(lines[end], " \t") != "---" && strings.TrimRight(lines[end], " \t") != "..." {
		end++
	}
	if end == len(lines) {
		return "", fmt.Errorf("YAML 前言缺少结束分隔符")
	}
	if metadata != nil {
		if _, err := metadataLookup(metadata, []string{setting}, 0); err != nil {
			return "", err
		}
		for i := 0; i < len(metadata.Content); i += 2 {
			key, old := metadata.Content[i], metadata.Content[i+1]
			if key.Value != setting {
				continue
			}
			// A YAML node encoder retains metadata values and comments for complex
			// existing root nodes; body Markdown remains byte-for-byte unchanged.
			if metadata.Style&yaml.FlowStyle != 0 || key.Column != 1 || key.Style != 0 || old.Anchor != "" || old.Kind != yaml.ScalarNode || old.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 || strings.Contains(old.Value, "\n") || key.Line != old.Line {
				old.Kind = yaml.ScalarNode
				old.Tag = "!!str"
				old.Value = directory
				old.Style = 0
				old.Content = nil
				old.Alias = nil
				encoded, err = yaml.Marshal(metadata)
				if err != nil {
					return "", err
				}
				return bom + rawLines[0] + strings.ReplaceAll(string(encoded), "\n", eol) + strings.Join(rawLines[end:], ""), nil
			}
			// Preserve unrelated front matter, body, and inline comments for the
			// normal single-line setting produced by the native folder chooser.
			line := lines[key.Line]
			prefix := line[:old.Column-1]
			comment := ""
			if old.LineComment != "" {
				comment = " " + old.LineComment
			}
			lineEOL := "\n"
			if strings.HasSuffix(rawLines[key.Line], "\r\n") {
				lineEOL = "\r\n"
			}
			rawLines[key.Line] = prefix + strings.TrimSuffix(string(encoded), "\n") + comment + lineEOL
			return bom + strings.Join(rawLines, ""), nil
		}
		if metadata.Style&yaml.FlowStyle != 0 {
			metadata.Content = append(metadata.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: setting}, value)
			encoded, err = yaml.Marshal(metadata)
			if err != nil {
				return "", err
			}
			return bom + rawLines[0] + strings.ReplaceAll(string(encoded), "\n", eol) + strings.Join(rawLines[end:], ""), nil
		}
	}
	return bom + strings.Join(rawLines[:end], "") + entry + eol + strings.Join(rawLines[end:], ""), nil
}

type ImageRootEdit struct {
	Original string `json:"original"`
	Content  string `json:"content"`
}

func (a *App) ImageLocation(id, reference string) (string, error) {
	media, err := a.ResolveMedia(id, reference)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(media)
	if err != nil || u.IsAbs() {
		return "", fmt.Errorf("网络图片没有本地文件位置")
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/files/"), "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("图片位置无效")
	}
	a.mu.Lock()
	root := a.roots[parts[0]]
	a.mu.Unlock()
	if root == "" {
		return "", fmt.Errorf("图片位置无效")
	}
	return filepath.Join(root, parts[1]), nil
}

func (a *App) ChooseImageRoot(id string) (*ImageRootEdit, error) {
	a.mu.Lock()
	d, ok := a.docs[id]
	if !ok {
		a.mu.Unlock()
		return nil, fmt.Errorf("文档不存在")
	}
	original, path := d.Content, d.Path
	a.mu.Unlock()
	base := ""
	if path != "" {
		base = filepath.Dir(path)
	}
	initial, _, err := documentImageRoot(original, base)
	if err != nil {
		return nil, err
	}
	if info, e := os.Stat(initial); e != nil || !info.IsDir() {
		initial = ""
	}
	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择图片存储路径", DefaultDirectory: initial})
	if err != nil || directory == "" {
		return nil, err
	}
	content, err := withDocumentImageRoot(original, directory)
	if err != nil {
		return nil, err
	}
	return &ImageRootEdit{Original: original, Content: content}, nil
}

func imagePathFromRoot(content, documentRoot, reference string) (string, error) {
	p := filepath.FromSlash(reference)
	// Drive-qualified, UNC and file-URI paths bypass YAML, just as existing
	// absolute image references survive the native Set Image Root operation.
	if filepath.IsAbs(p) {
		return p, nil
	}
	root, configured, err := documentImageRoot(content, documentRoot)
	if err != nil {
		return "", err
	}
	if root == "" {
		return "", fmt.Errorf("请先保存文档")
	}
	if configured {
		p = strings.TrimLeft(p, `/\`)
	}
	return filepath.Join(root, p), nil
}
