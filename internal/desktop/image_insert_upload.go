package desktop

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

func validateUploadedTemporaryReference(doc Document, base, reference, dir string) error {
	if u, err := url.Parse(reference); err == nil && strings.EqualFold(u.Scheme, "file") {
		resolved, resolveErr := uploadImagePath(doc, base, reference)
		if resolveErr == nil {
			rel, relErr := filepath.Rel(dir, resolved)
			if relErr == nil && filepath.IsLocal(rel) {
				return fmt.Errorf("上传服务返回了即将清理的临时文件地址，请配置服务返回持久图片地址")
			}
		}
	}
	return nil
}

// Upload runs without the document mutex; the caller's snapshot is checked again
// before returning a URL that the frontend may insert into the document.
func (a *App) uploadInsertedImage(doc Document, settings Settings, path string) (string, error) {
	if settings.ImageUploader != "custom" && settings.ImageUploader != "picgo-core" && settings.ImageUploader != "picgo-app" {
		return "", fmt.Errorf("请先在图像偏好设置中配置图片上传服务")
	}
	if _, err := readCopyImage(path); err != nil {
		return "", err
	}
	input := settings.ImageUploadCommand
	if settings.ImageUploader != "custom" {
		input = settings.ImageUploaderProgram
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	urls, output, err := runImageUploader(ctx, a.base, doc, settings.ImageUploader, input, []string{path})
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastImageUploadOutput = output
	if err != nil {
		return "", err
	}
	current := a.docs[doc.ID]
	if current == nil || current.Content != doc.Content || current.Path != doc.Path {
		return "", fmt.Errorf("图片已上传，但文档已变化，未插入返回地址；可查看上次上传输出")
	}
	return urls[0], nil
}
