package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExportYAMLVariablesPreserveValuesAndArgumentBoundaries(t *testing.T) {
	content := "\ufeff---\r\ntitle: '中文 & 引号 \" A'\r\nauthor: ''\r\nproject:\r\n  name: 项目甲\r\n  version: 2.1\r\ntags: [甲, 乙]\r\nactive: false\r\nliteral: '${currentPath}'\r\ncurrentFileName: 不能覆盖内置变量\r\nnullValue: null\r\nmultiline: |-\r\n  第一行\r\n  第二行\r\n---\r\n# 正文\r\n"
	doc := Document{Name: "源文件.md", Path: filepath.Join(t.TempDir(), "源文件.md"), Content: content}
	args := []string{"--metadata=title:${title}", "${author}", "${project.name}-${project.version}", "${tags}", "${tags.1}", "${active}", "${literal}", "${currentFileName}", "${nullValue}", "${multiline}", "${missing}"}
	actual, err := exportArguments(args, doc, "out.html")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"--metadata=title:中文 & 引号 \" A", "", "项目甲-2.1", "甲, 乙", "乙", "false", "${currentPath}", "源文件", "", "第一行\n第二行", "${missing}"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("argument expansion mismatch\n%q\n%q", actual, expected)
	}
	if doc.Content != content {
		t.Fatal("metadata expansion changed source")
	}
}

func TestExportYAMLAliasesMergeAndFailures(t *testing.T) {
	content := "---\ndefaults: &defaults\n  name: 默认\n  site: 中文\nproject:\n  <<: *defaults\n  name: 覆盖\nlist: &list [甲, 乙]\nalias: *list\n...\n正文"
	actual, err := exportArguments([]string{"${project.name}", "${project.site}", "${alias}"}, Document{Content: content}, "out.html")
	if err != nil || !reflect.DeepEqual(actual, []string{"覆盖", "中文", "甲, 乙"}) {
		t.Fatal(actual, err)
	}
	for _, text := range []string{"---\ntitle: [\n---\n", "---\ntitle: a\n", "---\ntitle: a\ntitle: b\n---\n", "---\ntitle: &loop [*loop]\n---\n"} {
		if _, err := exportArguments([]string{"${title}"}, Document{Content: text}, "out.html"); err == nil {
			t.Fatal("invalid or recursive metadata accepted", text)
		}
	}
	if _, err := exportArguments([]string{"${title}"}, Document{Content: "---\ntitle: '--output=other.txt'\n---"}, "out.html"); err == nil {
		t.Fatal("metadata bypassed output-path argument protection")
	}
}

func TestCustomExportYAMLReachesPandocAndFailurePreservesOutput(t *testing.T) {
	workspace, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	a.base = filepath.Join(workspace, "build", "bin")
	if _, err := os.Stat(a.pandoc()); err != nil {
		t.Skip("bundled Pandoc unavailable", err)
	}
	output := filepath.Join(a.dataDir, "元数据.html")
	a.docs["yaml"] = &Document{ID: "yaml", Name: "未保存.md", Content: "---\ntitle: 原标题\nproject:\n  name: 中文项目\n---\n\n正文来自编辑缓冲区。\n", Dirty: true}
	a.settings.CustomExports = []ExportPreset{{ID: "yaml", Name: "YAML HTML", Writer: "html5", Extension: "html", Arguments: []string{"--metadata=title:${project.name} ${title}"}}}
	a.settings.LastExport = "custom:yaml"
	a.settings.LastExportPath = output
	if _, err := a.ExportDialog("yaml", "custom:yaml", "", true); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "<title>中文项目 原标题</title>") {
		t.Fatal("expanded metadata did not reach Pandoc", string(before))
	}
	a.docs["yaml"].Content = "---\ntitle: [\n---\n正文"
	if _, err := a.ExportDialog("yaml", "custom:yaml", "", true); err == nil || !strings.Contains(err.Error(), "YAML") {
		t.Fatal("missing YAML error", err)
	}
	after, _ := os.ReadFile(output)
	if string(before) != string(after) {
		t.Fatal("invalid metadata overwrote old export")
	}
}

func TestExportYAMLRepeatedAliasExpansionIsBounded(t *testing.T) {
	content := "---\nx0: &x0 {name: value}\n"
	for i := 1; i < 24; i++ {
		content += fmt.Sprintf("x%d: &x%d {<<: [*x%d, *x%d]}\n", i, i, i-1, i-1)
	}
	content += "project: *x23\n---\n"
	if _, err := exportArguments([]string{"${project.missing}"}, Document{Content: content}, "out.html"); err == nil || !strings.Contains(err.Error(), "过大") {
		t.Fatal("repeated merge expansion did not stop", err)
	}
}
