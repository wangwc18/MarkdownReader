package desktop

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestYAMLUploadInsertionOptInAndClipboardCleanup(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "上传 文档.md")
	text := "---\ntypora-copy-images-to: upload\n---\n\n保留正文\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	png := clipboardPNG(t)
	source := filepath.Join(a.base, "原图.png")
	if err = os.WriteFile(source, png, 0644); err != nil {
		t.Fatal(err)
	}
	// The reserved action never becomes a directory, even with global copying.
	a.settings.CopyImages = true
	ref, err := a.insertLocalImage(d.ID, source)
	if err != nil || !strings.HasPrefix(ref, "assets/") {
		t.Fatalf("%q %v", ref, err)
	}
	if _, err = os.Stat(filepath.Join(a.base, "upload")); !os.IsNotExist(err) {
		t.Fatalf("reserved directory created: %v", err)
	}
	a.settings.AllowYAMLImageUpload = true
	if _, err = a.insertLocalImage(d.ID, source); err == nil {
		t.Fatal("missing service accepted")
	}
	t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
	exe, _ := os.Executable()
	a.settings.ImageUploader = "custom"
	a.settings.ImageUploadCommand = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- success`, exe)
	for _, clipboard := range []bool{false, true} {
		if clipboard {
			ref, err = a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png))
		} else {
			ref, err = a.insertLocalImage(d.ID, source)
		}
		if err != nil || ref != "https://example.invalid/image-0.png" {
			t.Fatalf("clipboard=%v %q %v", clipboard, ref, err)
		}
	}
	left, err := filepath.Glob(filepath.Join(a.dataDir, "image-insert-upload-*"))
	if err != nil || len(left) != 0 {
		t.Fatalf("temporary samples retained: %v %v", left, err)
	}
	if a.docs[d.ID].Content != text {
		t.Fatal("backend changed document")
	}
	got, err := os.ReadFile(source)
	if err != nil || string(got) != string(png) {
		t.Fatal("source changed", err)
	}
	a.settings.ImageUploadCommand = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- fail`, exe)
	if _, err = a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png)); err == nil {
		t.Fatal("upload failure accepted")
	}
	left, _ = filepath.Glob(filepath.Join(a.dataDir, "image-insert-upload-*"))
	if len(left) != 0 {
		t.Fatal("failed upload left temporary files", left)
	}
}

func TestYAMLUploadRejectsDocumentChangesWhileProcessRuns(t *testing.T) {
	for _, change := range []string{"content", "path", "close"} {
		t.Run(change, func(t *testing.T) {
			a := fixture(t)
			path := filepath.Join(a.base, "doc.md")
			original := "---\ntypora-copy-images-to: upload\n---\n\noriginal"
			if err := os.WriteFile(path, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			d, err := a.OpenPath(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
			exe, _ := os.Executable()
			ready, release := filepath.Join(a.base, "ready"), filepath.Join(a.base, "release")
			a.settings.AllowYAMLImageUpload = true
			a.settings.ImageUploader = "custom"
			a.settings.ImageUploadCommand = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- wait "%s" "%s"`, exe, ready, release)
			finished := make(chan error, 1)
			encoded := base64.StdEncoding.EncodeToString(clipboardPNG(t))
			go func() { _, err := a.ImportClipboardImage(d.ID, encoded); finished <- err }()
			defer os.WriteFile(release, []byte("release"), 0600)
			until := time.Now().Add(5 * time.Second)
			for {
				if _, err = os.Stat(ready); err == nil {
					break
				}
				if time.Now().After(until) {
					t.Fatal("uploader did not become ready")
				}
				time.Sleep(10 * time.Millisecond)
			}
			// Acquiring this mutex while the helper waits also checks that the long
			// upload does not prevent the document from being edited or closed.
			a.mu.Lock()
			switch change {
			case "content":
				a.docs[d.ID].Content = original + " edited"
			case "path":
				a.docs[d.ID].Path = filepath.Join(a.base, "renamed.md")
			case "close":
				delete(a.docs, d.ID)
			}
			a.mu.Unlock()
			if err = os.WriteFile(release, []byte("release"), 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-finished:
				if err == nil || !strings.Contains(err.Error(), "文档已变化") {
					t.Fatalf("%v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("upload did not finish")
			}
			if change == "content" && a.docs[d.ID].Content != original+" edited" {
				t.Fatal("new text overwritten")
			}
			left, _ := filepath.Glob(filepath.Join(a.dataDir, "image-insert-upload-*"))
			if len(left) != 0 {
				t.Fatal("temporary upload files retained", left)
			}
		})
	}
}

func TestSetImageUploadRulePreservesDocumentAndExplicitFolder(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	doc := a.docs[d.ID]
	doc.Content = "---\ntitle: 保留\ntypora-copy-images-to: assets\n---\n\n正文\r\n"
	result, err := a.SetImageUploadRule(d.ID)
	if err != nil || result.Original != doc.Content || !strings.HasSuffix(result.Content, "正文\r\n") || !strings.Contains(result.Content, "title: 保留") {
		t.Fatalf("%+v %v", result, err)
	}
	if folder, configured, err := documentImageCopyFolder(result.Content, a.base); err != nil || configured || folder != "" {
		t.Fatalf("%q %v %v", folder, configured, err)
	}
	if folder, configured, err := documentImageCopyFolder("---\ntypora-copy-images-to: ./upload\n---\n", a.base); err != nil || !configured || folder != filepath.Join(a.base, "upload") {
		t.Fatalf("explicit folder broken: %q %v %v", folder, configured, err)
	}
}
