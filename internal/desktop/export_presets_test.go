package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportPresetValidationAndSnapshots(t *testing.T) {
	preset := ExportPreset{ID: "test-id", Name: "中文导出", Writer: "gfm", Extension: "md", Arguments: []string{"--wrap=none"}}
	if err := validateExportPresets([]ExportPreset{preset}); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ExportPreset){func(p *ExportPreset) { p.Extension = "../md" }, func(p *ExportPreset) { p.Writer = "html --other" }, func(p *ExportPreset) { p.Name = " " }, func(p *ExportPreset) { p.Arguments = []string{"--output=elsewhere"} }, func(p *ExportPreset) { p.Arguments = []string{"-oelsewhere"} }} {
		bad := preset
		mutate(&bad)
		if validateExportPresets([]ExportPreset{bad}) == nil {
			t.Fatal("invalid preset accepted", bad)
		}
	}
	if validateExportPresets([]ExportPreset{preset, preset}) == nil {
		t.Fatal("duplicate preset IDs accepted")
	}
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	s := a.GetSettings()
	s.CustomExports = []ExportPreset{preset}
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	s.CustomExports[0].Arguments[0] = "changed"
	snapshot := a.GetSettings()
	snapshot.CustomExports[0].Name = "changed"
	snapshot.CustomExports[0].Arguments[0] = "changed"
	if a.GetSettings().CustomExports[0].Arguments[0] != "--wrap=none" || a.GetSettings().CustomExports[0].Name != "中文导出" {
		t.Fatal("settings alias modified live preset")
	}
	reopened := NewApp(a.base)
	if err := reopened.initStorage(); err != nil {
		t.Fatal(err)
	}
	if reopened.GetSettings().CustomExports[0].Name != "中文导出" {
		t.Fatal("preset did not persist")
	}
}

func TestCustomPandocExportUsesBufferAndPreservesOutputOnFailure(t *testing.T) {
	workspace, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	a := NewApp(root)
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	a.base = filepath.Join(workspace, "build", "bin")
	if _, err := os.Stat(a.pandoc()); err != nil {
		t.Skip("bundled Pandoc not present", err)
	}
	folder := filepath.Join(root, "中文 & 路径")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(folder, "源文档.md")
	if err := os.WriteFile(input, []byte("旧磁盘内容"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(folder, "输出 & 报告.html")
	preset := ExportPreset{ID: "html", Name: "自定义 HTML", Writer: "html5", Extension: "html", Arguments: []string{"--metadata=title:报告 ${currentFileName} & 引号", "--wrap=none"}}
	a.docs["test"] = &Document{ID: "test", Path: input, Name: filepath.Base(input), Content: "# 缓冲区标题\n\n尚未保存的内容。\n", Dirty: true}
	a.settings.CustomExports = []ExportPreset{preset}
	a.settings.LastExport = "custom:html"
	a.settings.LastExportPath = output
	if path, err := a.ExportDialog("test", "custom:html", "", true); err != nil || path != output {
		t.Fatal(path, err)
	}
	bytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bytes), "尚未保存的内容") || !strings.Contains(string(bytes), "报告 源文档 &amp; 引号") {
		t.Fatal("buffer or expanded argument missing", string(bytes))
	}
	disk, _ := os.ReadFile(input)
	if string(disk) != "旧磁盘内容" {
		t.Fatal("export modified original source")
	}
	a.settings.CustomExports[0].Writer = "markdownreader_no_such_writer"
	if _, err := a.ExportDialog("test", "custom:html", "", true); err == nil || !strings.Contains(err.Error(), "Pandoc") {
		t.Fatal("missing conversion error", err)
	}
	unchanged, _ := os.ReadFile(output)
	if string(unchanged) != string(bytes) {
		t.Fatal("failed export overwrote existing file")
	}
	leftovers, _ := filepath.Glob(filepath.Join(folder, ".markdownreader-export-*"))
	if len(leftovers) != 0 {
		t.Fatal("temporary output left behind", leftovers)
	}
	a.settings.CustomExports = nil
	if _, err := a.ExportDialog("test", "custom:html", "", true); err == nil {
		t.Fatal("deleted last export preset accepted")
	}
}
