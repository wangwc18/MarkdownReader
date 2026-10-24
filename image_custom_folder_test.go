package main

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalCustomFolderOutsideDocumentAndClipboard(t *testing.T) {
	a := fixture(t)
	docDir := filepath.Join(a.base, "documents")
	os.Mkdir(docDir, 0755)
	path := filepath.Join(docDir, "中文 文档.md")
	os.WriteFile(path, []byte("body"), 0644)
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	png := clipboardPNG(t)
	source := filepath.Join(a.base, "source.png")
	os.WriteFile(source, png, 0644)
	a.settings.ImageInsertAction = "copy"
	a.settings.ImageFolderCustom = true
	for _, folder := range []string{filepath.Join(a.base, "共享 #100%", "absolute"), "../sibling/${filename}.assets"} {
		a.settings.ImageFolder = folder
		target := folder
		if !filepath.IsAbs(target) {
			target = filepath.Join(docDir, strings.ReplaceAll(folder, "${filename}", "中文 文档"))
		}
		ref, err := a.insertLocalImage(d.ID, source)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := url.PathUnescape(ref)
		if err != nil {
			t.Fatal(err)
		}
		resolved := filepath.Clean(filepath.Join(docDir, filepath.FromSlash(decoded)))
		if resolved != filepath.Join(target, "source.png") {
			t.Fatalf("%q %q", resolved, target)
		}
		stored, err := os.ReadFile(resolved)
		if err != nil || string(stored) != string(png) {
			t.Fatal("copy bytes", err)
		}
		clip, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err = url.PathUnescape(clip)
		if err != nil {
			t.Fatal(err)
		}
		stored, err = os.ReadFile(filepath.Join(docDir, filepath.FromSlash(decoded)))
		if err != nil || string(stored) != string(png) {
			t.Fatal("clipboard bytes", err)
		}
	}
	a.settings.ImageFolder = "C:relative"
	if _, err = a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png)); err == nil {
		t.Fatal("ambiguous drive-relative path accepted")
	}
}
