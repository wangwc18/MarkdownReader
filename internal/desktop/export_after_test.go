package desktop

import (
	"path/filepath"
	"testing"
)

func TestExportAfterDirectoryPolicyAndPersistence(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "中文 & 输出")
	file := filepath.Join(folder, "报告.md")
	for _, test := range []struct {
		name         string
		global       bool
		preset       *ExportPreset
		output, want string
	}{
		{"built-in respects global enabled", true, nil, file, folder},
		{"built-in respects global disabled", false, nil, file, ""},
		{"legacy custom inherits enabled", true, &ExportPreset{}, file, folder},
		{"custom enabled overrides global disabled", false, &ExportPreset{AfterExport: "open-folder"}, file, folder},
		{"custom disabled overrides global enabled", true, &ExportPreset{AfterExport: "none"}, file, ""},
		{"folder output opens itself", false, &ExportPreset{Kind: "program", OutputMode: "folder", AfterExport: "open-folder"}, folder, folder},
		{"action without output never opens folder", true, &ExportPreset{Kind: "program", OutputMode: "none", AfterExport: "open-folder"}, "", ""},
		{"cancel without path never opens folder", true, nil, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exportAfterDirectory(Settings{OpenAfterExport: test.global}, test.preset, test.output); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	s := a.GetSettings()
	s.CustomExports = []ExportPreset{{ID: "after", Name: "导出后动作", Writer: "gfm", Extension: "md", AfterExport: "open-folder"}}
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	reopened := NewApp(a.base)
	if err := reopened.initStorage(); err != nil {
		t.Fatal(err)
	}
	if reopened.GetSettings().CustomExports[0].AfterExport != "open-folder" {
		t.Fatal("after-export preference did not persist")
	}
	s.CustomExports[0].AfterExport = "unknown-action"
	if err := a.SetSettings(s); err == nil {
		t.Fatal("unknown action accepted")
	}
	if a.GetSettings().CustomExports[0].AfterExport != "open-folder" {
		t.Fatal("invalid setting changed live state")
	}
}
