package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	win "golang.org/x/sys/windows"
)

type ImageUploadResult struct {
	Original   string            `json:"original"`
	References map[string]string `json:"references"`
	Uploaded   int               `json:"uploaded"`
	Skipped    int               `json:"skipped"`
	Output     string            `json:"output"`
}

func uploadImagePath(doc Document, fallback, reference string) (string, error) {
	reference, _, _ = strings.Cut(reference, "#")
	reference, _, _ = strings.Cut(reference, "?")
	path, err := url.PathUnescape(reference)
	if err != nil {
		return "", err
	}
	path = strings.TrimPrefix(path, "file:///")
	if !filepath.IsAbs(path) {
		base := fallback
		if doc.Path != "" {
			base = filepath.Dir(doc.Path)
		}
		path, err = imagePathFromRoot(doc.Content, base, path)
		if err != nil {
			return "", err
		}
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico":
	default:
		return "", fmt.Errorf("不支持的图片格式")
	}
	return path, nil
}

func uploadResultURLs(output string, count int) ([]string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if count < 1 || len(lines) < count {
		return nil, fmt.Errorf("上传程序返回的图片地址数量不足")
	}
	result := make([]string, count)
	for i, line := range lines[len(lines)-count:] {
		line = strings.TrimSpace(line)
		u, err := url.Parse(line)
		if err != nil || !((u.Scheme == "http" || u.Scheme == "https") && u.Host != "" || u.Scheme == "file" && (u.Path != "" || u.Host != "")) {
			return nil, fmt.Errorf("上传程序返回的第%d个地址无效", i+1)
		}
		result[i] = line
	}
	return result, nil
}

func runImageUpload(ctx context.Context, base string, doc Document, command string, paths []string) ([]string, string, error) {
	args, err := win.DecomposeCommandLine(strings.TrimSpace(command))
	if err != nil || len(args) == 0 {
		return nil, "", fmt.Errorf("请在图像偏好设置中填写有效的上传命令")
	}
	filename := ""
	if doc.Path != "" {
		filename = filepath.Base(doc.Path)
	}
	for i, arg := range args {
		args[i] = strings.NewReplacer("${filename}", filename, "${filepath}", doc.Path).Replace(arg)
	}
	program := args[0]
	if !filepath.IsAbs(program) && strings.ContainsAny(program, `/\`) {
		program = filepath.Join(base, program)
	}
	cmd := exec.CommandContext(ctx, program, append(args[1:], paths...)...)
	hideConversionConsole(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = base
	if doc.Path != "" {
		cmd.Dir = filepath.Dir(doc.Path)
	}
	stdout, stderr := &exportCapture{limit: 4 << 20}, &exportCapture{limit: 4 << 20}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = cmd.Run()
	text := decodeExportOutput(stdout.Bytes(), "auto")
	message := text
	if stderr.Len() > 0 {
		message += "\n" + decodeExportOutput(stderr.Bytes(), "auto")
	}
	if ctx.Err() != nil {
		return nil, message, fmt.Errorf("图片上传超时或取消：%w", ctx.Err())
	}
	if err != nil {
		summary := message
		if len(summary) > 4096 {
			summary = summary[:4096]
		}
		return nil, message, fmt.Errorf("图片上传程序失败：%s %w", summary, err)
	}
	urls, err := uploadResultURLs(text, len(paths))
	return urls, message, err
}

func (a *App) GetImageUploadOutput() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastImageUploadOutput
}

func (a *App) TestImageUploader(uploader, command string) (*ImageUploadResult, error) {
	if uploader != "picgo-app" && (uploader != "custom" && uploader != "picgo-core" || strings.TrimSpace(command) == "") {
		return nil, fmt.Errorf("请先填写上传命令或程序路径")
	}
	dir, err := os.MkdirTemp(a.dataDir, "image-upload-test-ll*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(dir)
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAACAAAAAYCAYAAACbU/80AAAAPElEQVR4AezQQQoAMAgDwdWX9+e2b1Chlw3kGAiTBWdSqDfvN/kcDyiggAIKLAhEQL8LBxjFAwoooIACFwAA//9lZw9LAAAABklEQVQDAPAbOCF7gZ39AAAAAElFTkSuQmCC")
	paths := []string{}
	for _, name := range []string{"sample-1.png", "sample-2.png"} {
		path := filepath.Join(dir, name)
		if err = os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}
		defer os.Remove(path)
		paths = append(paths, path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	urls, output, err := runImageUploader(ctx, a.base, Document{}, uploader, command, paths)
	a.mu.Lock()
	a.lastImageUploadOutput = output
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	result := &ImageUploadResult{References: map[string]string{}, Uploaded: len(paths), Output: output}
	for i, path := range paths {
		result.References[filepath.Base(path)] = urls[i]
	}
	return result, nil
}

func (a *App) UploadImages(id string, references []string) (*ImageUploadResult, error) {
	if len(references) == 0 || len(references) > 2000 {
		return nil, fmt.Errorf("请选择1至2000个图片引用")
	}
	a.mu.Lock()
	d := a.docs[id]
	if d == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("文档不存在")
	}
	snapshot := *d
	settings := a.settings
	fallback := a.roots[strings.Trim(strings.TrimPrefix(d.BaseURL, "/files/"), "/")]
	a.mu.Unlock()
	if settings.ImageUploader != "custom" && settings.ImageUploader != "picgo-core" && settings.ImageUploader != "picgo-app" {
		return nil, fmt.Errorf("请先在图像偏好设置中配置图片上传服务")
	}
	result := &ImageUploadResult{Original: snapshot.Content, References: map[string]string{}}
	paths := []string{}
	indices := map[string]int{}
	byReference := map[string]int{}
	seen := map[string]bool{}
	for _, ref := range references {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		u, err := url.Parse(ref)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "data" || u.Scheme == "blob") {
			result.Skipped++
			continue
		}
		path, err := uploadImagePath(snapshot, fallback, ref)
		if err != nil {
			return nil, err
		}
		if _, err = readCopyImage(path); err != nil {
			return nil, err
		}
		key := strings.ToLower(path)
		index, ok := indices[key]
		if !ok {
			index = len(paths)
			indices[key] = index
			paths = append(paths, path)
		}
		byReference[ref] = index
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("没有可上传的本地图片")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	input := settings.ImageUploadCommand
	if settings.ImageUploader != "custom" {
		input = settings.ImageUploaderProgram
	}
	urls, output, err := runImageUploader(ctx, a.base, snapshot, settings.ImageUploader, input, paths)
	a.mu.Lock()
	a.lastImageUploadOutput = output
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for ref, index := range byReference {
		result.References[ref] = urls[index]
	}
	result.Uploaded = len(paths)
	result.Output = output
	return result, nil
}
