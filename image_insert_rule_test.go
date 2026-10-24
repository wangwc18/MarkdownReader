package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentImageCopyRule(t *testing.T) {
	base := t.TempDir()
	for _, value := range []string{"images", "../outside", filepath.ToSlash(filepath.Join(base, "absolute"))} {
		text, err := withDocumentImageSetting("---\ntitle: 保留 # 注释\n---\n\n正文\r\n", "typora-copy-images-to", value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, "title: 保留 # 注释") || !strings.HasSuffix(text, "正文\r\n") {
			t.Fatal(text)
		}
		folder, configured, err := documentImageCopyFolder(text, base)
		want := filepath.FromSlash(value)
		if !filepath.IsAbs(want) {
			want = filepath.Join(base, want)
		}
		if err != nil || !configured || folder != want {
			t.Fatalf("%q %v %v", folder, configured, err)
		}
		changed, err := withDocumentImageSetting(text, "typora-copy-images-to", "next")
		if err != nil || strings.Count(changed, "typora-copy-images-to:") != 1 {
			t.Fatalf("%s %v", changed, err)
		}
	}
	for _, bad := range []string{"[]", "{}", "true", "123"} {
		if _, _, err := documentImageCopyFolder("---\ntypora-copy-images-to: "+bad+"\n---\n", base); err == nil {
			t.Fatal("accepted " + bad)
		}
	}
}

func TestInsertedImagesHonorDocumentCopyRule(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "rule.md")
	text := "---\ntypora-copy-images-to: rule-images\n---\n\n正文"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.settings.CopyImages = false
	source := filepath.Join(a.base, "source.png")
	png := clipboardPNG(t)
	if err = os.WriteFile(source, png, 0644); err != nil {
		t.Fatal(err)
	}
	ref, err := a.insertLocalImage(d.ID, source)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "rule-images/source.png" {
		t.Fatal(ref)
	}
	got, err := os.ReadFile(filepath.Join(a.base, "rule-images", "source.png"))
	if err != nil || string(got) != string(png) {
		t.Fatalf("%v", err)
	}
	clip, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png))
	if err != nil || !strings.HasPrefix(clip, "rule-images/clipboard-") {
		t.Fatalf("%q %v", clip, err)
	}
	if _, err = os.Stat(source); err != nil {
		t.Fatal("source changed", err)
	}
	if a.settings.CopyImages {
		t.Fatal("document rule changed global setting")
	}
}

func TestImageFolderExpandsCurrentDocumentFilename(t *testing.T) {
	a := fixture(t)
	a.settings.CopyImages = true
	a.settings.ImageFolder = "./${filename}.assets"
	for _, name := range []string{"中文 100%.md", "notes.v2.markdown", "second.md"} {
		path := filepath.Join(a.base, name)
		if err := os.WriteFile(path, []byte("正文"), 0644); err != nil {
			t.Fatal(err)
		}
		d, err := a.OpenPath(path)
		if err != nil {
			t.Fatal(err)
		}
		png := clipboardPNG(t)
		ref, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png))
		if err != nil {
			t.Fatal(err)
		}
		location, err := a.ImageLocation(d.ID, ref)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSuffix(name, filepath.Ext(name)) + ".assets"
		if filepath.Base(filepath.Dir(location)) != want {
			t.Fatalf("%s -> %s", name, location)
		}
		source := filepath.Join(a.base, "source.png")
		if err := os.WriteFile(source, png, 0644); err != nil {
			t.Fatal(err)
		}
		inserted, err := a.insertLocalImage(d.ID, source)
		if err != nil {
			t.Fatal(err)
		}
		insertedPath, err := a.ImageLocation(d.ID, inserted)
		if err != nil || filepath.Base(filepath.Dir(insertedPath)) != want {
			t.Fatalf("%s %v", insertedPath, err)
		}
	}
	if a.settings.ImageFolder != "./${filename}.assets" {
		t.Fatal("expanded folder leaked into global configuration")
	}
}
