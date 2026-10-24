package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	win "golang.org/x/sys/windows"
)

const picGoEndpoint = "http://127.0.0.1:36677/upload"

func (a *App) ChooseImageUploaderProgram() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 PicGo 程序", Filters: []runtime.FileFilter{{DisplayName: "Windows 程序", Pattern: "*.exe"}}})
}

func picGoCoreCommand(program string) (string, error) {
	if strings.TrimSpace(program) == "" {
		return "", fmt.Errorf("请设置 PicGo-Core 程序路径")
	}
	return win.ComposeCommandLine([]string{program, "upload"}), nil
}

func postPicGoUpload(ctx context.Context, endpoint string, paths []string) ([]string, string, error) {
	data, err := json.Marshal(map[string][]string{"list": paths})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > 4<<20 {
		return nil, "", fmt.Errorf("PicGo 返回数据超过4MB")
	}
	output := string(body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, output, fmt.Errorf("PicGo 上传接口返回 HTTP %d", response.StatusCode)
	}
	var result struct {
		Success bool     `json:"success"`
		Result  []string `json:"result"`
		Message string   `json:"message"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, output, fmt.Errorf("PicGo 返回的数据不是有效 JSON：%w", err)
	}
	if !result.Success {
		return nil, output, fmt.Errorf("PicGo 上传失败：%s", result.Message)
	}
	if len(result.Result) != len(paths) {
		return nil, output, fmt.Errorf("PicGo 返回的图片地址数量不匹配")
	}
	for _, address := range result.Result {
		if strings.ContainsAny(address, "\r\n") {
			return nil, output, fmt.Errorf("PicGo 图片地址不能包含换行")
		}
	}
	urls, err := uploadResultURLs(strings.Join(result.Result, "\n"), len(paths))
	return urls, output, err
}

func dialFailure(err error) bool {
	var operation *net.OpError
	return errors.As(err, &operation) && operation.Op == "dial"
}

func runPicGoApplication(ctx context.Context, base, program string, paths []string) ([]string, string, error) {
	urls, output, err := postPicGoUpload(ctx, picGoEndpoint, paths)
	if err == nil || !dialFailure(err) {
		return urls, output, err
	}
	if strings.TrimSpace(program) == "" {
		return nil, output, fmt.Errorf("无法连接 PicGo，请启动应用并启用 127.0.0.1:36677 服务：%w", err)
	}
	if !filepath.IsAbs(program) {
		program = filepath.Join(base, program)
	}
	cmd := exec.Command(program)
	hideConversionConsole(cmd)
	cmd.Dir = filepath.Dir(program)
	if err = cmd.Start(); err != nil {
		return nil, output, fmt.Errorf("无法启动 PicGo：%w", err)
	}
	go cmd.Wait()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		select {
		case <-startup.Done():
			return nil, output, fmt.Errorf("等待 PicGo 上传接口启动超时：%w", startup.Err())
		case <-time.After(250 * time.Millisecond):
		}
		urls, output, err = postPicGoUpload(ctx, picGoEndpoint, paths)
		if err == nil || !dialFailure(err) {
			return urls, output, err
		}
	}
}

func runImageUploader(ctx context.Context, base string, doc Document, service, input string, paths []string) ([]string, string, error) {
	switch service {
	case "custom":
		return runImageUpload(ctx, base, doc, input, paths)
	case "picgo-core":
		command, err := picGoCoreCommand(input)
		if err != nil {
			return nil, "", err
		}
		return runImageUpload(ctx, base, doc, command, paths)
	case "picgo-app":
		return runPicGoApplication(ctx, base, input, paths)
	default:
		return nil, "", fmt.Errorf("请先在图像偏好设置中配置图片上传服务")
	}
}
