package main

import (
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClipboardImageReferenceUsesConfiguredRoot(t *testing.T) {
	a := fixture(t)
	root := filepath.Join(a.base, "图片根")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.base, "doc.md")
	text := "---\ntypora-root-url: '" + filepath.ToSlash(root) + "'\n---\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(clipboardPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	location, err := a.ImageLocation(d.ID, reference)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(location); err != nil {
		t.Fatal(err)
	}
	if a.settings.RelativeImages && !strings.HasPrefix(reference, "../") {
		t.Fatalf("reference ignored configured image root: %s", reference)
	}
}

func TestDocumentImageRootResolution(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "图片根目录")
	content := "---\ntitle: 保留标题\ntypora-root-url: '" + filepath.ToSlash(root) + "'\n---\n正文"
	for _, ref := range []string{"assets/图.svg", "/assets/图.svg"} {
		got, err := imagePathFromRoot(content, "", ref)
		if err != nil || got != filepath.Join(root, "assets", "图.svg") {
			t.Fatalf("absolute YAML root without document path: %q %v", got, err)
		}
	}
	other := filepath.Join(base, "other.png")
	got, err := imagePathFromRoot(content, "", other)
	if err != nil || got != other {
		t.Fatalf("absolute reference changed: %q %v", got, err)
	}
	got, err = imagePathFromRoot("---\ntypora-root-url: ../图片\n---\n", filepath.Join(base, "文档"), "a.svg")
	if err != nil || got != filepath.Join(base, "图片", "a.svg") {
		t.Fatalf("relative root resolution: %q %v", got, err)
	}
	for _, text := range []string{"---\ntypora-root-url: [a, b]\n---\n", "---\ntypora-root-url: 7\n---\n", "---\ntypora-root-url: ../图片\n---\n", "---\ntitle: [\n---\n"} {
		if _, err := imagePathFromRoot(text, "", "a.png"); err == nil {
			t.Fatalf("invalid or unresolved root accepted: %q", text)
		}
	}
}

func TestUpdateImageRootPreservesDocument(t *testing.T) {
	directory := `C:\图片目录\a # b`
	for _, source := range []string{
		"正文\r\n![图](a.png)\n尾行",
		"\ufeff---\r\ntitle: '原文' # comment\r\n---\r\n\r\n正文\n尾行",
		"---\ntitle: 原文\ntypora-root-url: old # retain\n---\n\n正文\r\n尾行",
		"---\n{title: 原文, typora-root-url: old, author: 作者}\n---\n正文\r\n",
		"---\n{title: 原文}\n---\n正文\r\n",
		"---\ntypora-root-url: |\n  old\n  path\ntitle: 原文\n---\n正文\r\n",
	} {
		result, err := withDocumentImageRoot(source, directory)
		if err != nil {
			t.Fatal(err)
		}
		root, configured, err := documentImageRoot(result, "")
		if err != nil || !configured || root != directory {
			t.Fatalf("bad updated root: %q %v\n%s", root, err, result)
		}
		body := "正文\r\n"
		if strings.Contains(source, "尾行") {
			body = source[strings.Index(source, "正文"):]
		}
		if !strings.HasSuffix(result, body) {
			t.Fatalf("body changed: %q", result)
		}
		if strings.Contains(source, "title:") && !strings.Contains(result, "原文") {
			t.Fatal("title lost")
		}
		if strings.Contains(source, "# retain") && !strings.Contains(result, "# retain") {
			t.Fatal("inline comment lost")
		}
		if strings.HasPrefix(source, "\ufeff") && !strings.HasPrefix(result, "\ufeff") {
			t.Fatal("BOM lost")
		}
	}
	if _, err := withDocumentImageRoot("---\ntypora-root-url: a\ntypora-root-url: b\n---\nbody", directory); err == nil {
		t.Fatal("duplicate root accepted")
	}
}

func TestResolveMediaUsesDocumentYAMLRoot(t *testing.T) {
	a := fixture(t)
	root := filepath.Join(a.base, "图片")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "图.svg"), []byte("root image"), 0644); err != nil {
		t.Fatal(err)
	}
	d := a.NewDocument()
	text := "---\ntypora-root-url: '" + filepath.ToSlash(root) + "'\n---\n\n![图](/assets/图.svg)\n"
	if err := a.UpdateDocument(d.ID, text, "LF", 1); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"/assets/图.svg", "assets/图.svg#中文"} {
		location, err := a.ImageLocation(d.ID, ref)
		expected, canonicalErr := filepath.EvalSymlinks(filepath.Join(root, "assets", "图.svg"))
		if err != nil || canonicalErr != nil || location != expected {
			t.Fatalf("wrong image location: %q %v", location, err)
		}
		media, err := a.ResolveMedia(d.ID, ref)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", strings.Split(media, "#")[0], nil))
		if w.Code != 200 || w.Body.String() != "root image" {
			t.Fatalf("wrong root media: %d %s", w.Code, w.Body.String())
		}
	}
	if a.docs[d.ID].Content != text {
		t.Fatal("resolution modified Markdown")
	}
	other := a.NewDocument()
	if _, err := a.ResolveMedia(other.ID, "assets/图.svg"); err == nil {
		t.Fatal("image root leaked to another document")
	}
}
