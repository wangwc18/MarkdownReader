package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type ExportPreset struct {
	Kind           string   `json:"kind,omitempty"`
	Program        string   `json:"program,omitempty"`
	OutputMode     string   `json:"outputMode,omitempty"`
	OutputEncoding string   `json:"outputEncoding,omitempty"`
	ShowOutput     bool     `json:"showOutput,omitempty"`
	AfterExport    string   `json:"afterExport,omitempty"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Writer         string   `json:"writer"`
	Extension      string   `json:"extension"`
	Arguments      []string `json:"arguments"`
}

func cloneExportPresets(input []ExportPreset) []ExportPreset {
	if input == nil {
		return nil
	}
	result := append([]ExportPreset{}, input...)
	for i := range result {
		result[i].Arguments = append([]string(nil), result[i].Arguments...)
	}
	return result
}

var exportPresetID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var exportWriter = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_+.-]*$`)
var exportExtension = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func validateExportPresets(presets []ExportPreset) error {
	seen := map[string]bool{}
	for _, preset := range presets {
		if !exportPresetID.MatchString(preset.ID) || seen[preset.ID] {
			return fmt.Errorf("自定义导出标识无效或重复")
		}
		seen[preset.ID] = true
		if preset.AfterExport != "" && preset.AfterExport != "none" && preset.AfterExport != "open-folder" {
			return fmt.Errorf("无效的导出后动作")
		}
		if strings.TrimSpace(preset.Name) == "" || (!(preset.Kind == "program" && (preset.OutputMode == "none" || preset.OutputMode == "folder")) && !exportExtension.MatchString(preset.Extension)) {
			return fmt.Errorf("自定义导出的名称、Pandoc 格式或扩展名无效")
		}
		if preset.Kind != "" && preset.Kind != "pandoc" && preset.Kind != "program" {
			return fmt.Errorf("未知的自定义导出类型")
		}
		if preset.Kind == "program" {
			if preset.OutputEncoding != "" && preset.OutputEncoding != "auto" {
				if _, err := canonicalEncoding(preset.OutputEncoding); err != nil {
					return err
				}
			}
			if strings.TrimSpace(preset.Program) == "" || strings.ContainsRune(preset.Program, 0) {
				return fmt.Errorf("请填写有效的转换程序路径或名称")
			}
			if preset.OutputMode != "" && preset.OutputMode != "file" && preset.OutputMode != "stdout" && preset.OutputMode != "none" && preset.OutputMode != "folder" {
				return fmt.Errorf("无效的转换程序输出方式")
			}
		} else if !exportWriter.MatchString(preset.Writer) {
			return fmt.Errorf("无效的 Pandoc 输出格式")
		}
		for _, arg := range preset.Arguments {
			if strings.ContainsRune(arg, 0) {
				return fmt.Errorf("自定义导出参数不能包含空字符")
			}
			if preset.Kind != "program" && (arg == "--" || strings.HasPrefix(arg, "-o") || arg == "--output" || strings.HasPrefix(arg, "--output=")) {
				return fmt.Errorf("自定义导出的输出路径由保存对话框指定，附加参数不能设置 -o/--output")
			}
		}
	}
	return nil
}

func customExport(s Settings, format string) (*ExportPreset, error) {
	if !strings.HasPrefix(format, "custom:") {
		return nil, nil
	}
	if err := validateExportPresets(s.CustomExports); err != nil {
		return nil, err
	}
	for _, preset := range s.CustomExports {
		if preset.ID == strings.TrimPrefix(format, "custom:") {
			return &preset, nil
		}
	}
	return nil, fmt.Errorf("自定义导出项已被删除，请选择其他导出格式")
}

var exportVariable = regexp.MustCompile(`\$\{([^{}]+)\}`)

func exportArguments(args []string, doc Document, output string) ([]string, error) {
	return expandExportArguments(args, doc, output, nil, true)
}

func expandExportArguments(args []string, doc Document, output string, additional map[string]string, pandoc bool) ([]string, error) {
	folder := ""
	if doc.Path != "" {
		folder = filepath.Dir(doc.Path)
	}
	values := map[string]string{"currentPath": doc.Path, "currentFolder": folder, "currentFileName": strings.TrimSuffix(doc.Name, filepath.Ext(doc.Name)), "currentFileFullName": doc.Name, "outputFileName": strings.TrimSuffix(filepath.Base(output), filepath.Ext(output)), "outputFileFullName": filepath.Base(output), "today": time.Now().Format("2006-01-02")}
	if output == "" {
		values["outputFileName"] = ""
		values["outputFileFullName"] = ""
	}
	for key, value := range additional {
		values[key] = value
	}
	needsMetadata := false
	for _, arg := range args {
		for _, match := range exportVariable.FindAllStringSubmatch(arg, -1) {
			if _, ok := values[match[1]]; !ok {
				needsMetadata = true
			}
		}
	}
	var metadataError error
	var rootMetadata *yaml.Node
	if needsMetadata {
		rootMetadata, metadataError = exportMetadata(doc.Content)
		if metadataError != nil {
			return nil, metadataError
		}
	}
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = exportVariable.ReplaceAllStringFunc(arg, func(token string) string {
			key := token[2 : len(token)-1]
			if value, ok := values[key]; ok {
				return value
			}
			node, err := metadataLookup(rootMetadata, strings.Split(key, "."), 0)
			if err != nil {
				metadataError = err
				return token
			}
			if node == nil {
				return token
			}
			value, err := metadataText(node, 0)
			if err != nil {
				metadataError = err
				return token
			}
			return value
		})
	}
	if metadataError != nil {
		return nil, metadataError
	}
	checkPreset := ExportPreset{ID: "expanded", Name: "参数", Writer: "gfm", Extension: "md", Arguments: result}
	if !pandoc {
		checkPreset.Kind = "program"
		checkPreset.Program = "converter"
	}
	if err := validateExportPresets([]ExportPreset{checkPreset}); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) convertWithPandoc(content, inputPath, writer, resources, output string, extra []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	args := []string{"-f", "markdown+tex_math_dollars+yaml_metadata_block+footnotes+strikeout+superscript+subscript", "-t", writer, "--standalone"}
	args = append(args, extra...)
	args = append(args, "--resource-path", resources, "-o", output)
	cmd := exec.CommandContext(ctx, a.pandoc(), args...)
	hideConversionConsole(cmd)
	cmd.Stdin = strings.NewReader(content)
	if inputPath != "" {
		cmd.Dir = filepath.Dir(inputPath)
	}
	message, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Pandoc 导出失败：%s %w", message, err)
	}
	return nil
}
