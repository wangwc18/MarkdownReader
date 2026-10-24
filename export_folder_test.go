package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgramFolderExportsBufferAndMergesExistingDirectory(t *testing.T) {
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "中文 & 输出")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(destination, "报告.md"), []byte("旧报告"), 0600)
	os.WriteFile(filepath.Join(destination, "keep.txt"), []byte("保留原文件"), 0600)
	doc := Document{ID: "folder", Name: "未保存.md", Content: "# 未保存缓冲区", Dirty: true}
	a.docs[doc.ID] = &doc
	preset := ExportPreset{ID: "folder", Name: "文件夹导出", Kind: "program", Program: os.Args[0], OutputMode: "folder", Arguments: []string{"-test.run=^TestExportProgramFixture$", "markdownreader-export-fixture", "folder", "${outputPath}", "${inputPath}", "${outputFileFullName}"}}
	a.settings.CustomExports = []ExportPreset{preset}
	a.settings.LastExport = "custom:folder"
	a.settings.LastExportPath = destination
	if path, err := a.ExportDialog(doc.ID, "custom:folder", "", true); err != nil || path != destination {
		t.Fatal(path, err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "报告.md"))
	if err != nil || string(data) != "中文 & 输出\n# 未保存缓冲区" {
		t.Fatal(string(data), err)
	}
	data, err = os.ReadFile(filepath.Join(destination, "keep.txt"))
	if err != nil || string(data) != "保留原文件" {
		t.Fatal(string(data), err)
	}
	data, err = os.ReadFile(filepath.Join(destination, "资源", "图片.txt"))
	if err != nil || string(data) != "资源内容" {
		t.Fatal(string(data), err)
	}
	if !doc.Dirty || doc.Content != "# 未保存缓冲区" || a.GetExportOutput() != "文件夹导出完成" {
		t.Fatal("document or log changed", doc, a.GetExportOutput())
	}
	preset.Arguments[2] = "folder-fail"
	a.settings.CustomExports = []ExportPreset{preset}
	if _, err := a.ExportDialog(doc.ID, "custom:folder", "", true); err == nil {
		t.Fatal("converter failure accepted")
	}
	data, _ = os.ReadFile(filepath.Join(destination, "报告.md"))
	if string(data) != "中文 & 输出\n# 未保存缓冲区" {
		t.Fatal("failed converter changed output")
	}
	entries, _ := os.ReadDir(destination)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".markdownreader-") {
			t.Fatal("transaction remains", entry.Name())
		}
	}
}

func TestFolderExportRollsBackEarlierReplacement(t *testing.T) {
	stage, err := newExportStage()
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	defer stage.Cleanup()
	stage.root.WriteFile("a.txt", []byte("new a"), 0600)
	stage.root.WriteFile("b.txt", []byte("new b"), 0600)
	destination := t.TempDir()
	os.WriteFile(filepath.Join(destination, "a.txt"), []byte("old a"), 0600)
	blocked := filepath.Join(destination, "b.txt")
	os.WriteFile(blocked, []byte("old b"), 0600)
	if err := os.Chmod(blocked, 0400); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blocked, 0600)
	if err := mergeExportFolder(stage, destination); err == nil {
		t.Fatal("readonly replacement unexpectedly succeeded")
	}
	for name, want := range map[string]string{"a.txt": "old a", "b.txt": "old b"} {
		data, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil || string(data) != want {
			t.Fatal(name, string(data), err)
		}
	}
	entries, _ := os.ReadDir(destination)
	if len(entries) != 2 {
		t.Fatal("rollback left extra files", entries)
	}
}

func TestFolderExportRejectsDestinationJunctionBeforeChanges(t *testing.T) {
	stage, err := newExportStage()
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	defer stage.Cleanup()
	stage.root.WriteFile("a.txt", []byte("new"), 0600)
	stage.root.Mkdir("linked", 0700)
	stage.root.WriteFile("linked/sentinel.txt", []byte("new"), 0600)
	destination, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(destination, "a.txt"), []byte("old"), 0600)
	os.WriteFile(filepath.Join(outside, "sentinel.txt"), []byte("untouched"), 0600)
	link := filepath.Join(destination, "linked")
	if err := testDirectoryJunction(link, outside); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)
	if err := mergeExportFolder(stage, destination); err == nil {
		t.Fatal("destination junction accepted")
	}
	for path, want := range map[string]string{filepath.Join(destination, "a.txt"): "old", filepath.Join(outside, "sentinel.txt"): "untouched"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatal(path, string(data), err)
		}
	}
}
