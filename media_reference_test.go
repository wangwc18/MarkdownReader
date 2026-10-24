package main

import (
	"bytes"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageReferencesEscapeNamesAndResolveQueries(t *testing.T) {
	for _, relative := range []bool{true, false} {
		t.Run(map[bool]string{true: "relative", false: "absolute"}[relative], func(t *testing.T) {
			a := fixture(t)
			root := filepath.Join(a.base, "中文 文档")
			folder := filepath.Join(root, "素材 #100%")
			if err := os.MkdirAll(folder, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "文档.md")
			if err := os.WriteFile(path, []byte("# 图片\n"), 0644); err != nil {
				t.Fatal(err)
			}
			picture := filepath.Join(folder, "图片 (1)#100%+&.png")
			data := clipboardPNG(t)
			if err := os.WriteFile(picture, data, 0644); err != nil {
				t.Fatal(err)
			}
			d, err := a.OpenPath(path)
			if err != nil {
				t.Fatal(err)
			}
			reference, err := imageReference(root, picture, relative)
			if err != nil {
				t.Fatal(err)
			}
			for _, escaped := range []string{"%23", "%25", "%20"} {
				if !strings.Contains(reference, escaped) {
					t.Fatalf("missing URI escape %s: %s", escaped, reference)
				}
			}
			media, err := a.ResolveMedia(d.ID, reference+"?v=1&lang=zh#中文视图")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(media, "?v=1&lang=zh#") {
				t.Fatalf("query or fragment lost: %s", media)
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("GET", media, nil))
			if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) {
				t.Fatalf("media mismatch: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestClipboardReferenceWithSpecialDirectory(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "文档.md")
	if err := os.WriteFile(path, []byte("# 图片\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.settings.ImageFolder = "截图 #100%"
	data := clipboardPNG(t)
	reference, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reference, "%23") || !strings.Contains(reference, "%25") {
		t.Fatalf("unsafe reference: %s", reference)
	}
	media, err := a.ResolveMedia(d.ID, reference)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", media, nil))
	if w.Code != 200 || !bytes.Equal(data, w.Body.Bytes()) {
		t.Fatal("clipboard reference did not resolve original image")
	}
}

func TestRelativeMediaSurvivesMovingDocumentFolder(t *testing.T) {
	a := fixture(t)
	oldRoot, newRoot := filepath.Join(a.base, "原文档目录"), filepath.Join(a.base, "移动后目录")
	if err := os.MkdirAll(oldRoot, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(oldRoot, "文档.md")
	if err := os.WriteFile(path, []byte("# 图片\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.settings.ImageFolder = "图片 #100%"
	data := clipboardPNG(t)
	reference, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatal(err)
	}
	text := "# 图片\n\n![图](" + reference + ")\n"
	if strings.Contains(reference, "../") {
		t.Fatalf("child image escaped its document folder: %s", reference)
	}
	if err := a.UpdateDocument(d.ID, text, "LF", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveDocument(d.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.CloseDocument(d.ID); err != nil {
		t.Fatal(err)
	}
	// Both paths are children of this test's disposable application directory.
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	moved, err := a.OpenPath(filepath.Join(newRoot, "文档.md"))
	if err != nil {
		t.Fatal(err)
	}
	if moved.Content != text {
		t.Fatal("moving document changed Markdown reference")
	}
	media, err := a.ResolveMedia(moved.ID, reference)
	if err != nil {
		t.Fatalf("reference=%q moved=%q relative=%v: %v", reference, moved.Path, a.settings.RelativeImages, err)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", media, nil))
	if w.Code != 200 || !bytes.Equal(data, w.Body.Bytes()) {
		t.Fatal("moved document cannot load original image")
	}
}
