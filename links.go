package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type LinkTarget struct {
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	URL      string `json:"url,omitempty"`
	Fragment string `json:"fragment,omitempty"`
}

func resolveLink(root, reference string) (LinkTarget, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return LinkTarget{}, errors.New("链接地址为空")
	}
	// A drive letter is a Windows path, rather than a URL protocol.
	if filepath.IsAbs(reference) && !strings.HasPrefix(reference, "file:") {
		return localLink(reference, "")
	}
	u, err := url.Parse(reference)
	if err != nil {
		return LinkTarget{}, err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return LinkTarget{}, errors.New("网址缺少主机名")
		}
		return LinkTarget{Kind: "url", URL: u.String()}, nil
	case "mailto":
		return LinkTarget{Kind: "url", URL: u.String()}, nil
	case "", "file":
	default:
		return LinkTarget{}, errors.New("不支持打开此链接协议")
	}
	if u.Path == "" && u.Host == "" {
		return LinkTarget{Kind: "anchor", Fragment: u.Fragment}, nil
	}
	path := filepath.FromSlash(u.Path)
	if u.Scheme == "file" {
		if u.Host != "" && u.Host != "localhost" {
			path = `\\` + u.Host + path
		} else if len(path) > 2 && path[0] == '\\' && path[2] == ':' {
			path = path[1:]
		}
	} else if u.Host != "" {
		return LinkTarget{}, errors.New("本地链接应使用文件路径或 file URL")
	}
	if !filepath.IsAbs(path) {
		if root == "" {
			return LinkTarget{}, errors.New("请先保存文档，再打开相对路径链接")
		}
		path = filepath.Join(root, path)
	}
	return localLink(path, u.Fragment)
}

func localLink(path, fragment string) (LinkTarget, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return LinkTarget{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return LinkTarget{}, err
	}
	if info.IsDir() {
		return LinkTarget{Kind: "folder", Path: path, Fragment: fragment}, nil
	}
	kind := "file"
	if isMarkdown(path) {
		return LinkTarget{Kind: "markdown", Path: path, Fragment: fragment}, nil
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico", ".pdf", ".txt", ".csv", ".html", ".htm", ".mp3", ".mp4", ".ogg", ".wav":
	default:
		return LinkTarget{}, errors.New("不支持自动打开此文件类型")
	}
	return LinkTarget{Kind: kind, Path: path, Fragment: fragment}, nil
}

func (a *App) ResolveLink(id, reference string) (LinkTarget, error) {
	a.mu.Lock()
	d, ok := a.docs[id]
	root := ""
	if ok && d.Path != "" {
		root = filepath.Dir(d.Path)
	}
	a.mu.Unlock()
	if !ok {
		return LinkTarget{}, errors.New("文档不存在")
	}
	return resolveLink(root, reference)
}

func (a *App) OpenLink(id, reference string) error {
	target, err := a.ResolveLink(id, reference)
	if err != nil {
		return err
	}
	if target.Kind == "url" {
		runtime.BrowserOpenURL(a.ctx, target.URL)
		return nil
	}
	if target.Path != "" {
		u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(target.Path)}
		runtime.BrowserOpenURL(a.ctx, u.String())
	}
	return nil
}
