package desktop

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type exportCapture struct {
	bytes.Buffer
	limit int
}

func (b *exportCapture) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("转换程序输出过大")
	}
	return b.Buffer.Write(p)
}

func (a *App) GetExportOutput() string { a.mu.Lock(); defer a.mu.Unlock(); return a.lastExportOutput }

func decodeExportOutput(data []byte, name string) string {
	if len(data) == 0 {
		return ""
	}
	if name == "" {
		name = "auto"
	}
	text, _, err := decodeFileAs(data, name)
	if err != nil {
		return fmt.Sprintf("[命令输出无法按 %s 显示，共 %d 字节；请选择对应的命令输出编码]", name, len(data))
	}
	return text
}

func (a *App) programBytes(doc Document, preset ExportPreset, finalPath string) ([]byte, string, error) {
	output, err := os.CreateTemp(filepath.Dir(finalPath), ".markdownreader-export-*"+filepath.Ext(finalPath))
	if err != nil {
		return nil, "", err
	}
	output.Close()
	defer os.Remove(output.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	message, err := a.runExportProgram(ctx, doc, preset, output.Name(), finalPath)
	if err != nil {
		return nil, message, err
	}
	data, err := os.ReadFile(output.Name())
	if err != nil {
		return nil, message, fmt.Errorf("转换程序未生成输出文件：%s %w", message, err)
	}
	return data, message, nil
}

func (a *App) runExportProgram(ctx context.Context, doc Document, preset ExportPreset, outputPath, finalPath string) (string, error) {
	input, err := os.CreateTemp(a.dataDir, "converter-input-*"+filepath.Ext(doc.Name))
	if err != nil {
		return "", err
	}
	defer os.Remove(input.Name())
	if _, err = input.WriteString(doc.Content); err != nil {
		input.Close()
		return "", err
	}
	if err = input.Close(); err != nil {
		return "", err
	}
	args, err := expandExportArguments(preset.Arguments, doc, finalPath, map[string]string{"inputPath": input.Name(), "outputPath": outputPath}, false)
	if err != nil {
		return "", err
	}
	program := preset.Program
	if !filepath.IsAbs(program) && strings.ContainsAny(program, `/\`) {
		program = filepath.Join(a.base, program)
	}
	if outputPath != "" && preset.OutputMode != "folder" {
		if err = os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	cmd := exec.CommandContext(ctx, program, args...)
	hideConversionConsole(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = a.base
	if doc.Path != "" {
		cmd.Dir = filepath.Dir(doc.Path)
	}
	cmd.Stdin = strings.NewReader(doc.Content)
	stdout := &exportCapture{limit: 64 << 20}
	stderr := &exportCapture{limit: 4 << 20}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	message := decodeExportOutput(stdout.Bytes(), preset.OutputEncoding)
	if stderr.Len() > 0 {
		if message != "" {
			message += "\n"
		}
		message += decodeExportOutput(stderr.Bytes(), preset.OutputEncoding)
	}
	if ctx.Err() != nil {
		return message, fmt.Errorf("转换程序超时：%w", ctx.Err())
	}
	if err != nil {
		return message, fmt.Errorf("转换程序失败：%s %w", message, err)
	}
	if preset.OutputMode == "stdout" {
		if err = os.WriteFile(outputPath, stdout.Bytes(), 0600); err != nil {
			return message, err
		}
	}
	return message, nil
}
