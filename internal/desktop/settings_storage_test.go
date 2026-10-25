package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSettingsProcessWriter(t *testing.T) {
	root, field := os.Getenv("MARKDOWNREADER_SETTINGS_TEST_ROOT"), os.Getenv("MARKDOWNREADER_SETTINGS_TEST_FIELD")
	if root == "" {
		t.Skip("subprocess helper")
	}
	a := NewApp(root)
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	s := a.GetSettings()
	if err := os.WriteFile(filepath.Join(root, field+".ready"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "start")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("start timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if field == "theme" {
		s.Theme = "Night"
	} else {
		s.FontSize = 25
	}
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
}

func TestTwoProcessesDoNotLoseSettings(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	results := make(chan error, 2)
	for _, field := range []string{"theme", "font"} {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSettingsProcessWriter$", "-test.v")
		cmd.Env = append(os.Environ(), "MARKDOWNREADER_SETTINGS_TEST_ROOT="+root, "MARKDOWNREADER_SETTINGS_TEST_FIELD="+field)
		go func() {
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Log(string(output))
			}
			results <- err
		}()
	}
	for {
		_, one := os.Stat(filepath.Join(root, "theme.ready"))
		_, two := os.Stat(filepath.Join(root, "font.ready"))
		if one == nil && two == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("writers did not initialise")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.WriteFile(filepath.Join(root, "start"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	a := NewApp(root)
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	if a.settings.Theme != "Night" || a.settings.FontSize != 25 {
		t.Fatalf("process write lost: %+v", a.settings)
	}
}

func TestIndependentWindowsMergeSettings(t *testing.T) {
	a := fixture(t)
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	first, second := a.GetSettings(), b.GetSettings()
	first.Theme = "Night"
	first.Markdown.SmartQuotes = true
	first.Shortcuts["save"] = "Ctrl+Alt+S"
	second.FontSize = 22
	second.Markdown.SmartDashes = true
	second.Shortcuts["open"] = "Ctrl+Alt+O"
	var group sync.WaitGroup
	for instance, settings := range map[*App]Settings{a: first, b: second} {
		group.Add(1)
		go func(app *App, s Settings) {
			defer group.Done()
			if err := app.SetSettings(s); err != nil {
				t.Error(err)
			}
		}(instance, settings)
	}
	group.Wait()
	c := NewApp(a.base)
	if err := c.initStorage(); err != nil {
		t.Fatal(err)
	}
	s := c.GetSettings()
	if s.Theme != "Night" || s.FontSize != 22 || !s.Markdown.SmartQuotes || !s.Markdown.SmartDashes || len(s.Shortcuts) != 2 {
		t.Fatalf("independent settings lost: %+v", s)
	}
}

func TestSettingsMergePreservesEncodingChangesAndRecentFiles(t *testing.T) {
	a := fixture(t)
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	a.rememberEncoding("first.md", "GBK")
	a.settings.Recent = []string{"first.md"}
	if err := a.persistSettings(); err != nil {
		t.Fatal(err)
	}
	b.rememberEncoding("second.md", "Big5")
	b.settings.Recent = []string{"second.md"}
	if err := b.persistSettings(); err != nil {
		t.Fatal(err)
	}
	if len(b.settings.FileEncodings) != 2 || len(b.settings.Recent) != 2 {
		t.Fatalf("metadata lost: %+v", b.settings)
	}
	// Removing one known encoding must not erase an unseen addition.
	a.rememberEncoding("first.md", "auto")
	if err := a.persistSettings(); err != nil {
		t.Fatal(err)
	}
	if len(a.settings.FileEncodings) != 1 || a.settings.FileEncodings[encodingPathKey("second.md")] != "Big5" {
		t.Fatalf("encoding removal lost unrelated key: %+v", a.settings.FileEncodings)
	}
}

func TestSettingsFailurePreservesMemoryAndCorruptFile(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.dataDir, "settings.json")
	if err := os.WriteFile(path, []byte("broken JSON"), 0644); err != nil {
		t.Fatal(err)
	}
	s := a.GetSettings()
	s.FontSize = 24
	if err := a.SetSettings(s); err == nil {
		t.Fatal("corrupt settings overwritten")
	}
	if a.GetSettings().FontSize != 16 {
		t.Fatal("failed persistence changed active settings")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "broken JSON" {
		t.Fatal("original corrupt file changed")
	}
	copy := a.GetSettings()
	copy.Shortcuts["save"] = "changed"
	if len(a.GetSettings().Shortcuts) != 0 {
		t.Fatal("snapshot shares mutable shortcuts")
	}
}

func TestFrontendSnapshotDoesNotRevertBackgroundMerge(t *testing.T) {
	a := fixture(t)
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	baseline := b.GetSettings()
	s := a.GetSettings()
	s.Theme = "Night"
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	b.settings.Recent = []string{"opened-in-background.md"}
	if err := b.persistSettings(); err != nil {
		t.Fatal(err)
	}
	local := baseline
	local.FontSize = 23
	if err := b.SetSettingsFrom(local, baseline); err != nil {
		t.Fatal(err)
	}
	if b.settings.Theme != "Night" || b.settings.FontSize != 23 || len(b.settings.Recent) != 1 {
		t.Fatalf("stale frontend reverted background merge: %+v", b.settings)
	}
}

func TestSettingsRefreshAdoptsOtherWindowWithoutWriting(t *testing.T) {
	a := fixture(t)
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	s := a.GetSettings()
	s.Theme = "Night"
	s.FontSize = 21
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.dataDir, "settings.json")
	before, _ := os.ReadFile(path)
	next, err := b.RefreshSettings()
	if err != nil || next == nil || next.Theme != "Night" || next.FontSize != 21 {
		t.Fatalf("refresh failed: %+v %v", next, err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("refresh wrote settings")
	}
	next, err = b.RefreshSettings()
	if err != nil || next != nil {
		t.Fatalf("unchanged refresh: %+v %v", next, err)
	}
	if err := os.WriteFile(path, []byte("null"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RefreshSettings(); err == nil {
		t.Fatal("accepted invalid settings")
	}
	if b.settings.Theme != "Night" {
		t.Fatal("failed refresh changed settings")
	}
}
