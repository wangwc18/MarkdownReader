package desktop

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportProgramFixture(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "markdownreader-export-fixture" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Skip("subprocess converter fixture")
	}
	args := os.Args[index+1:]
	if len(args) < 4 {
		os.Exit(9)
	}
	mode, output, input, title := args[0], args[1], args[2], args[3]
	if mode == "sleep" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(10)
	}
	snapshot, err := os.ReadFile(input)
	if err != nil || string(snapshot) != string(stdin) {
		os.Exit(11)
	}
	if mode == "missing" {
		fmt.Print("未产生文件")
		os.Exit(0)
	}
	if mode == "junction" {
		if err := testDirectoryJunction(filepath.Join(output, "linked"), title); err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(15)
		}
		os.Exit(0)
	}
	if mode == "folder" || mode == "folder-fail" {
		if err := os.MkdirAll(filepath.Join(output, "资源"), 0700); err != nil {
			os.Exit(16)
		}
		if err := os.WriteFile(filepath.Join(output, "报告.md"), append([]byte(title+"\n"), stdin...), 0600); err != nil {
			os.Exit(17)
		}
		if err := os.WriteFile(filepath.Join(output, "资源", "图片.txt"), []byte("资源内容"), 0600); err != nil {
			os.Exit(18)
		}
		fmt.Print("文件夹导出完成")
		if mode == "folder-fail" {
			os.Exit(7)
		}
		os.Exit(0)
	}
	if mode == "action" {
		if output != "" {
			os.Exit(14)
		}
		fmt.Print("执行：" + title)
		os.Exit(0)
	}
	if strings.HasPrefix(mode, "encoded:") {
		encoded, err := encodeDocument(&Document{Content: "中文完成", EOL: "LF", Encoding: strings.TrimPrefix(mode, "encoded:")}, false)
		if err != nil {
			os.Exit(13)
		}
		os.Stdout.Write(encoded)
		os.Exit(0)
	}
	if mode == "fail" {
		_ = os.WriteFile(output, []byte("不完整输出"), 0600)
		fmt.Fprint(os.Stderr, "转换故障中文")
		os.Exit(7)
	}
	data := []byte(title + "\n" + string(stdin))
	if mode == "stdout" {
		os.Stdout.Write(data)
	} else {
		if err := os.WriteFile(output, data, 0600); err != nil {
			os.Exit(12)
		}
		fmt.Print("转换成功：" + title)
	}
	os.Exit(0)
}

func TestCustomProgramExportFileStdoutAndFailureProtection(t *testing.T) {
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(a.base, "中文 & 空格")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(folder, "源文件.md")
	if err := os.WriteFile(input, []byte("旧磁盘内容"), 0600); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: '中文 & \"引号\" %NAME%! 保留'\n---\n\n未保存正文。\n"
	a.docs["program"] = &Document{ID: "program", Name: "源文件.md", Path: input, Content: content, Dirty: true}
	preset := ExportPreset{ID: "program", Kind: "program", Name: "自定义程序", Program: os.Args[0], Extension: "txt", Arguments: []string{"-test.run=^TestExportProgramFixture$", "markdownreader-export-fixture", "file", "${outputPath}", "${inputPath}", "${title}"}}
	if err := validateExportPresets([]ExportPreset{preset}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(folder, "输出 & 报告.txt")
	a.settings.LastExport = "custom:program"
	a.settings.LastExportPath = output
	for _, mode := range []string{"file", "stdout"} {
		preset.Arguments[2] = mode
		preset.OutputMode = mode
		a.settings.CustomExports = []ExportPreset{preset}
		if _, err := a.ExportDialog("program", "custom:program", "", true); err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != "中文 & \"引号\" %NAME%! 保留\n"+content {
			t.Fatal("converter altered argument boundaries or input", string(actual))
		}
		if a.GetExportOutput() == "" {
			t.Fatal("command output missing")
		}
	}
	before, _ := os.ReadFile(output)
	preset.OutputMode = "file"
	for _, mode := range []string{"fail", "missing"} {
		preset.Arguments[2] = mode
		a.settings.CustomExports = []ExportPreset{preset}
		if _, err := a.ExportDialog("program", "custom:program", "", true); err == nil {
			t.Fatal("failed/missing converter output accepted", mode)
		}
		after, _ := os.ReadFile(output)
		if string(after) != string(before) {
			t.Fatal("failed converter replaced prior output")
		}
	}
	disk, _ := os.ReadFile(input)
	if string(disk) != "旧磁盘内容" {
		t.Fatal("converter overwrote source")
	}
	files, _ := filepath.Glob(filepath.Join(a.dataDir, "converter-input-*"))
	if len(files) > 0 {
		t.Fatal("snapshot leaked", files)
	}
	files, _ = filepath.Glob(filepath.Join(folder, ".markdownreader-export-*"))
	if len(files) > 0 {
		t.Fatal("staging output leaked", files)
	}
	preset.Arguments[2] = "sleep"
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := a.runExportProgram(ctx, *a.docs["program"], preset, filepath.Join(folder, "timeout.txt"), output); err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatal("converter timeout missing", err)
	}
}

func TestExportProgramOutputEncodingsKeepRawFileBytes(t *testing.T) {
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	doc := Document{Name: "未保存.md", Content: "测试内容"}
	for _, encoding := range []string{"UTF-8", "GBK", "GB18030", "UTF-16 LE", "UTF-16 BE", "Big5"} {
		preset := ExportPreset{Kind: "program", Program: os.Args[0], OutputMode: "stdout", OutputEncoding: encoding, Arguments: []string{"-test.run=^TestExportProgramFixture$", "markdownreader-export-fixture", "encoded:" + encoding, "${outputPath}", "${inputPath}", "unused"}}
		data, message, err := a.programBytes(doc, preset, filepath.Join(a.dataDir, "output.txt"))
		if err != nil {
			t.Fatal(encoding, err)
		}
		expected, err := encodeDocument(&Document{Content: "中文完成", EOL: "LF", Encoding: encoding}, false)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != string(expected) || message != "中文完成" {
			t.Fatalf("%s: bytes changed or message garbled: %q", encoding, message)
		}
		if encoding != "Big5" && decodeExportOutput(data, "auto") != "中文完成" {
			t.Fatal("automatic decoding failed", encoding)
		}
	}
	if !strings.Contains(decodeExportOutput([]byte{0, 1, 2, 3}, "UTF-8"), "请选择") {
		t.Fatal("invalid text did not provide encoding guidance")
	}
	if validateExportPresets([]ExportPreset{{ID: "bad", Name: "bad", Kind: "program", Program: "converter", Extension: "txt", OutputEncoding: "not-an-encoding"}}) == nil {
		t.Fatal("unknown output encoding accepted")
	}
}

func TestExportProgramWithoutOutputSkipsDialogAndKeepsSource(t *testing.T) {
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: 中文动作\n---\n\n未保存内容。\n"
	a.docs["action"] = &Document{ID: "action", Name: "未保存.md", Content: content, Dirty: true}
	preset := ExportPreset{ID: "action", Name: "仅执行动作", Kind: "program", Program: os.Args[0], OutputMode: "none", Arguments: []string{"-test.run=^TestExportProgramFixture$", "markdownreader-export-fixture", "action", "${outputPath}", "${inputPath}", "${title}|${outputFileName}|${outputFileFullName}"}}
	if err := validateExportPresets([]ExportPreset{preset}); err != nil {
		t.Fatal(err)
	}
	a.settings.CustomExports = []ExportPreset{preset}
	// No Wails context is present: entering a native save dialog would fail.
	for _, repeat := range []bool{false, true} {
		path, err := a.ExportDialog("action", "custom:action", "", repeat)
		if err != nil || path != "" {
			t.Fatal(path, err)
		}
		if a.GetExportOutput() != "执行：中文动作||" {
			t.Fatal("missing action execution or nonempty output variables", a.GetExportOutput())
		}
		if a.settings.LastExport != "custom:action" || a.settings.LastExportPath != "" {
			t.Fatal("last action configuration incorrect")
		}
	}
	if a.docs["action"].Content != content || !a.docs["action"].Dirty {
		t.Fatal("action modified document")
	}
	files, _ := filepath.Glob(filepath.Join(a.dataDir, "converter-input-*"))
	if len(files) != 0 {
		t.Fatal("action snapshot leaked")
	}
}
