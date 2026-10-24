package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageCopyAndFilesystemHistory(t *testing.T) {
	a := fixture(t)
	root := filepath.Join(a.base, "images")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "图 #1.png")
	original := clipboardPNG(t)
	if err := os.WriteFile(source, original, 0644); err != nil {
		t.Fatal(err)
	}
	d := a.NewDocument()
	content := "---\ntypora-root-url: '" + filepath.ToSlash(root) + "'\n---\n"
	if err := a.UpdateDocument(d.ID, content, "LF", 1); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(a.base, "副本 #2.png")
	result, err := a.copyImageTo(d.ID, "图%20%231.png", target, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.Original != content || !strings.HasPrefix(result.Reference, "/../") {
		t.Fatalf("copy result: %+v", result)
	}
	if a.docs[d.ID].Content != content {
		t.Fatal("backend copy changed document before frontend transaction")
	}
	for _, path := range []string{source, target} {
		b, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(b, original) {
			t.Fatalf("copy bytes: %s %v", path, err)
		}
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("native-compatible undo did not remove new copy")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("undo touched source")
	}
	if err := a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(target); err != nil || !bytes.Equal(b, original) {
		t.Fatal("redo did not recreate exact copy")
	}
	other := a.NewDocument()
	if err := a.SetImageCopyState(other.ID, result.Token, false); err == nil {
		t.Fatal("history token crossed documents")
	}
	if err := os.WriteFile(target, []byte("external change"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err == nil {
		t.Fatal("undo removed externally edited file")
	}
	if b, _ := os.ReadFile(target); string(b) != "external change" {
		t.Fatal("external file damaged")
	}
}

func TestImageCopyOverwriteNoopAndFailure(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	source := filepath.Join(a.base, "source.png")
	target := filepath.Join(a.base, "target.png")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.copyImageTo(d.ID, source, target, false); err == nil {
		t.Fatal("unconfirmed overwrite accepted")
	}
	r, err := a.copyImageTo(d.ID, source, target, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetImageCopyState(d.ID, r.Token, false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "existing" {
		t.Fatal("overwrite undo lost original destination")
	}
	if err := a.SetImageCopyState(d.ID, r.Token, true); err != nil {
		t.Fatal(err)
	}
	noop, err := a.copyImageTo(d.ID, source, source, false)
	if err != nil || noop.Token != "" {
		t.Fatalf("same-file copy: %+v %v", noop, err)
	}
	if _, err := a.copyImageTo(d.ID, source, filepath.Join(a.base, "missing", "copy.png"), false); err == nil {
		t.Fatal("missing directory accepted")
	}
	if err := a.CloseDocument(d.ID); err != nil {
		t.Fatal(err)
	}
	if len(a.imageCopies) != 0 {
		t.Fatal("closed document retained image snapshots")
	}
}
