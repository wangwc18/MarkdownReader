package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictModeDefaultLegacyAndPersistence(t *testing.T) {
	a := NewApp(t.TempDir())
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	if !a.GetSettings().Markdown.StrictMode {
		t.Fatal("new profiles must preserve strict parser default")
	}
	if !a.GetSettings().Markdown.MathLineBreaks {
		t.Fatal("new profile must enable observed math line-break default")
	}
	s := a.GetSettings()
	s.Markdown.StrictMode = false
	s.Markdown.MathLineBreaks = false
	s.Markdown.MathNumbering = true
	s.Markdown.PlainHTMLMathLatex = true
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	reopened := NewApp(a.base)
	if err := reopened.initStorage(); err != nil {
		t.Fatal(err)
	}
	if reopened.GetSettings().Markdown.StrictMode {
		t.Fatal("explicit loose mode did not persist")
	}
	if reopened.GetSettings().Markdown.MathLineBreaks {
		t.Fatal("explicit disabled math line-break preference did not persist")
	}
	if !reopened.GetSettings().Markdown.MathNumbering {
		t.Fatal("explicit math numbering preference did not persist")
	}
	if !reopened.GetSettings().Markdown.PlainHTMLMathLatex {
		t.Fatal("explicit LaTeX HTML preference did not persist")
	}
	legacy := NewApp(t.TempDir())
	if err := os.MkdirAll(legacy.dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy.dataDir, "settings.json"), []byte(`{"markdown":{"mark":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := legacy.initStorage(); err != nil {
		t.Fatal(err)
	}
	if !legacy.GetSettings().Markdown.StrictMode {
		t.Fatal("old profiles silently enabled loose mode")
	}
	if !legacy.GetSettings().Markdown.MathLineBreaks {
		t.Fatal("legacy settings lost math line-break default")
	}
	if legacy.GetSettings().Markdown.Mark {
		t.Fatal("legacy fixture was not loaded")
	}
	if legacy.GetSettings().Markdown.MathNumbering || NewApp(t.TempDir()).GetSettings().Markdown.MathNumbering {
		t.Fatal("new and legacy profiles must default to observed disabled numbering")
	}
	if legacy.GetSettings().Markdown.PlainHTMLMathLatex || NewApp(t.TempDir()).GetSettings().Markdown.PlainHTMLMathLatex {
		t.Fatal("new and legacy profiles must default to SVG HTML math")
	}
}
