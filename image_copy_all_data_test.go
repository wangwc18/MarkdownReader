package main

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBulkDataImagesKeepLabelsBytesAndHistory(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	png := clipboardPNG(t)
	one := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	two := "data:image/svg+xml," + url.PathEscape(`<svg xmlns="http://www.w3.org/2000/svg"><text>what? #中文</text></svg>`)
	dir := filepath.Join(a.base, "copies")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	result, err := a.copyImagesTo(d.ID, []string{one, one + "#view", two}, dir, map[string]string{one: "内嵌 图片", two: "矢量"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 2 || result.Skipped != 0 || result.Token == "" {
		t.Fatalf("%+v", result)
	}
	if result.References[one]+"#view" != result.References[one+"#view"] {
		t.Fatal(result.References)
	}
	for ref, want := range map[string]string{one: string(png), two: `<svg xmlns="http://www.w3.org/2000/svg"><text>what? #中文</text></svg>`} {
		path, err := a.ImageLocation(d.ID, result.References[ref])
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("bytes changed %v", err)
		}
		if ref == one && filepath.Base(path) != "内嵌 图片.png" {
			t.Fatal(path)
		}
	}
	if strings.Contains(result.References[two], "?") {
		t.Fatal("data payload query became URI suffix")
	}
	if err = a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("undo retained data copies")
	}
	if err = a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("redo lost data copies")
	}
}

func TestBulkInvalidDataImageLeavesNoFiles(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	one := "data:image/png;base64," + base64.StdEncoding.EncodeToString(clipboardPNG(t))
	dir := filepath.Join(a.base, "copies")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.copyImagesTo(d.ID, []string{one, "data:image/png;base64,not-base64"}, dir); err == nil {
		t.Fatal("invalid data accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed data batch partially wrote files")
	}
}
