package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
)

type imageCopySource struct {
	Key, Name, LocalPath string
	Data                 []byte
}

func decodeDataImage(reference string) ([]byte, string, error) {
	if len(reference) < 5 || !strings.EqualFold(reference[:5], "data:") {
		return nil, "", fmt.Errorf("不是内嵌图片")
	}
	content, _, _ := strings.Cut(reference[5:], "#")
	header, payload, ok := strings.Cut(content, ",")
	if !ok || len(header) > 1024 || len(payload) > 60*1024*1024 {
		return nil, "", fmt.Errorf("内嵌图片格式无效或超过大小限制")
	}
	parts := strings.Split(header, ";")
	mimeType := strings.ToLower(parts[0])
	ext, ok := map[string]string{"image/png": ".png", "image/jpeg": ".jpeg", "image/jpg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp", "image/svg+xml": ".svg", "image/x-icon": ".ico", "image/vnd.microsoft.icon": ".ico"}[mimeType]
	if !ok {
		return nil, "", fmt.Errorf("不支持的内嵌图片类型")
	}
	encoded := false
	for _, part := range parts[1:] {
		if strings.EqualFold(part, "base64") {
			encoded = true
		} else if !strings.Contains(part, "=") {
			return nil, "", fmt.Errorf("内嵌图片参数无效")
		}
	}
	var data []byte
	var err error
	if encoded {
		payload, err = url.PathUnescape(payload)
		if err != nil {
			return nil, "", fmt.Errorf("内嵌图片数据无效")
		}
		if base64.StdEncoding.DecodedLen(len(payload)) > 20*1024*1024+3 {
			return nil, "", fmt.Errorf("内嵌图片不能超过20MB")
		}
		data, err = base64.StdEncoding.DecodeString(payload)
	} else {
		var decoded string
		decoded, err = url.PathUnescape(payload)
		data = []byte(decoded)
	}
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("内嵌图片数据无效")
	}
	if len(data) > 20*1024*1024 {
		return nil, "", fmt.Errorf("内嵌图片不能超过20MB")
	}
	return data, ext, nil
}

func dataImageFilename(label, extension string) string {
	var name strings.Builder
	for _, r := range label {
		if name.Len() > 180 {
			break
		}
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			name.WriteByte('_')
		} else {
			name.WriteRune(r)
		}
	}
	result := strings.Trim(name.String(), " .")
	if result == "" {
		result = "image"
	}
	stem := strings.ToUpper(strings.SplitN(result, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
		result = "image_" + result
	}
	if !strings.HasSuffix(strings.ToLower(result), extension) {
		result += extension
	}
	return result
}

func (a *App) imageSourceForCopy(id, reference, label string) (*imageCopySource, error) {
	if u, err := url.Parse(reference); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return downloadHTTPImage(reference, label)
	}
	if len(reference) >= 5 && strings.EqualFold(reference[:5], "data:") {
		data, extension, err := decodeDataImage(reference)
		if err != nil {
			return nil, err
		}
		return &imageCopySource{Key: "data:" + extension + ":" + digest(data), Name: dataImageFilename(label, extension), Data: data}, nil
	}
	path, err := a.ImageLocation(id, reference)
	if err != nil {
		return nil, err
	}
	data, err := readCopyImage(path)
	if err != nil {
		return nil, err
	}
	return &imageCopySource{Key: strings.ToLower(path), Name: filepath.Base(path), LocalPath: path, Data: data}, nil
}
