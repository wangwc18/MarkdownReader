package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func importedFixture(t *testing.T) (*App, *Document, string) {
	t.Helper()
	a := fixture(t)
	d := a.newDoc()
	d.ImportID = uuid.NewString()
	d.Dirty = true
	root, _ := a.importRoot(d.ImportID)
	folder := filepath.Join(root, importAssetPrefix(d.ImportID), "media")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "图像.png"), clipboardPNG(t), 0644); err != nil {
		t.Fatal(err)
	}
	d.Content = "![图片](" + filepath.ToSlash(filepath.Join(importAssetPrefix(d.ImportID), "media", "图像.png")) + ")\n"
	d.BaseURL = a.registerRoot(root)
	return a, d, root
}

func TestImportedMediaSurvivesSaveWithoutMarkdownRewrite(t *testing.T) {
	a, d, root := importedFixture(t)
	original := d.Content
	id := d.ImportID
	folder := filepath.Join(a.base, "中文 保存目录")
	if err := os.Mkdir(folder, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "导入.md")
	if err := a.saveTo(d, path, false); err != nil {
		t.Fatal(err)
	}
	if d.ImportID != "" || d.Dirty || d.Content != original {
		t.Fatalf("save metadata/content: %+v", d)
	}
	for _, base := range []string{root, folder} {
		b, err := os.ReadFile(filepath.Join(base, importAssetPrefix(id), "media", "图像.png"))
		if err != nil || digest(b) != digest(clipboardPNG(t)) {
			t.Fatalf("missing media under %s: %v", base, err)
		}
	}
	reference := filepath.ToSlash(filepath.Join(importAssetPrefix(id), "media", "图像.png"))
	if _, err := a.ResolveMedia(d.ID, reference); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != original {
		t.Fatal("import save rewrote Markdown")
	}
}

func TestImportedMediaCollisionDoesNotOverwriteDocument(t *testing.T) {
	a, d, _ := importedFixture(t)
	folder := filepath.Join(a.base, "destination")
	media := filepath.Join(folder, importAssetPrefix(d.ImportID), "media")
	if err := os.MkdirAll(media, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "图像.png"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "existing.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.saveTo(d, path, false); err == nil {
		t.Fatal("asset collision accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "original" || d.Path != "" || d.ImportID == "" {
		t.Fatal("failed import save changed document")
	}
}

func TestImportedDraftRestoresMediaRoot(t *testing.T) {
	a, d, _ := importedFixture(t)
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	restored := b.RestoreDocument(*d)
	reference := filepath.ToSlash(filepath.Join(importAssetPrefix(d.ImportID), "media", "图像.png"))
	if _, err := b.ResolveMedia(restored.ID, reference); err != nil {
		t.Fatal(err)
	}
	if _, err := b.importRoot("../outside"); err == nil {
		t.Fatal("unsafe import id accepted")
	}
}

func TestImportedDraftRemainsPortableWhenProgramDirectoryMoves(t *testing.T) {
	_, d, root := importedFixture(t)
	moved := NewApp(t.TempDir())
	if err := moved.initStorage(); err != nil {
		t.Fatal(err)
	}
	newRoot, err := moved.importRoot(d.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.CopyFS(newRoot, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	restored := moved.RestoreDocument(*d)
	reference := filepath.ToSlash(filepath.Join(importAssetPrefix(d.ImportID), "media", "图像.png"))
	url, err := moved.ResolveMedia(restored.ID, reference)
	if err != nil {
		t.Fatal(err)
	}
	if url == "" || restored.BaseURL == d.BaseURL {
		t.Fatal("restored draft kept stale media registration")
	}
	if restored.Content != d.Content || restored.ImportID != d.ImportID {
		t.Fatal("moving draft changed Markdown")
	}
}
