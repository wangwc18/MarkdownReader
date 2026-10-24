package main

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

func downloadHTTPImage(reference, label string) (*imageCopySource, error) {
	u, err := url.Parse(reference)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("网络图片地址无效")
	}
	u.Fragment = ""
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("网络图片请求无效")
	}
	client := &http.Client{CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 || next.URL.Scheme != "http" && next.URL.Scheme != "https" {
			return fmt.Errorf("网络图片重定向无效或过多")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("无法下载图片，请检查网络连接或重定向")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("下载图片返回 HTTP %d", response.StatusCode)
	}
	if response.ContentLength > 20<<20 {
		return nil, fmt.Errorf("网络图片不能超过20MB")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (20<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("读取网络图片失败")
	}
	if len(data) == 0 || len(data) > 20<<20 {
		return nil, fmt.Errorf("网络图片为空或超过20MB")
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	mediaType = strings.ToLower(mediaType)
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = http.DetectContentType(data)
	}
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpeg", "image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp", "image/svg+xml": ".svg", "image/x-icon": ".ico", "image/vnd.microsoft.icon": ".ico"}[mediaType]
	if ext == "" {
		return nil, fmt.Errorf("网络资源不是支持的图片类型")
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" || name == "" {
		name = label
	}
	if current := strings.ToLower(path.Ext(name)); current == ext || mediaType == "image/jpeg" && (current == ".jpg" || current == ".jpeg") {
		ext = current
	}
	return &imageCopySource{Key: "http:" + u.String(), Name: dataImageFilename(name, ext), Data: data}, nil
}
