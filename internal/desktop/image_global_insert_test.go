package desktop

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalImageInsertionActionsAndDocumentPriority(t *testing.T) {
	a := fixture(t)
	if !a.settings.ApplyLocalImageRules {
		t.Fatal("local rules must be enabled for migrated settings")
	}
	path := filepath.Join(a.base, "plain.md")
	if err := os.WriteFile(path, []byte("plain document"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(a.base, "source.png")
	png := clipboardPNG(t)
	if err = os.WriteFile(source, png, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
	exe, _ := os.Executable()
	a.settings.ImageUploader = "custom"
	a.settings.ImageUploadCommand = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- success`, exe)
	a.settings.ImageInsertAction = "upload"
	for _, clipboard := range []bool{false, true} {
		var ref string
		if clipboard {
			ref, err = a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png))
		} else {
			ref, err = a.insertLocalImage(d.ID, source)
		}
		if err != nil || ref != "https://example.invalid/image-0.png" {
			t.Fatalf("clipboard=%v %q %v", clipboard, ref, err)
		}
	}
	a.settings.ApplyLocalImageRules = false
	ref, err := a.insertLocalImage(d.ID, source)
	if err != nil || ref != "source.png" {
		t.Fatalf("disabled global rules %q %v", ref, err)
	}
	ref, err = a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(png))
	if err != nil || !strings.HasPrefix(ref, "assets/clipboard-") {
		t.Fatalf("clipboard fallback %q %v", ref, err)
	}
	a.docs[d.ID].Content = "---\ntypora-copy-images-to: document-images\n---\nplain"
	ref, err = a.insertLocalImage(d.ID, source)
	if err != nil || ref != "document-images/source.png" {
		t.Fatalf("document folder priority %q %v", ref, err)
	}
	a.docs[d.ID].Content = "---\ntypora-copy-images-to: upload\n---\nplain"
	a.settings.AllowYAMLImageUpload = true
	ref, err = a.insertLocalImage(d.ID, source)
	if err != nil || !strings.HasPrefix(ref, "https:") {
		t.Fatalf("document upload priority %q %v", ref, err)
	}
	a.settings.AllowYAMLImageUpload = false
	a.settings.ApplyLocalImageRules = true
	a.settings.ImageInsertAction = "copy"
	ref, err = a.insertLocalImage(d.ID, source)
	if err != nil || ref != "assets/source.png" {
		t.Fatalf("disabled YAML uses global copy %q %v", ref, err)
	}
	a.settings.ImageInsertAction = "invalid"
	if err = a.SetSettings(a.settings); err == nil {
		t.Fatal("invalid action accepted")
	}
}

func TestLegacyCopySettingsKeepLocalRulesOnLoad(t *testing.T) {
	a := fixture(t)
	if err := os.WriteFile(filepath.Join(a.dataDir, "settings.json"), []byte(`{"copyImages":true,"imageFolder":"old-assets"}`), 0644); err != nil {
		t.Fatal(err)
	}
	reopened := NewApp(a.base)
	if err := reopened.initStorage(); err != nil {
		t.Fatal(err)
	}
	action, err := imageInsertionAction("plain document", reopened.settings)
	if err != nil || action != "copy" || !reopened.settings.ApplyLocalImageRules || reopened.settings.ImageFolder != "old-assets" {
		t.Fatalf("legacy settings changed: %q %+v %v", action, reopened.settings, err)
	}
}
