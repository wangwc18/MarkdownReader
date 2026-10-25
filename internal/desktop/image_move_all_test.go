package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestBatchMoveDeduplicatesAndPreservesCollisionsAndHistory(t *testing.T) {
	a := fixture(t)
	data := clipboardPNG(t)
	for _, dir := range []string{"one", "two", "dest"} {
		if err := os.Mkdir(filepath.Join(a.base, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"one/a.png", "two/a.png", "dest/stay.png"} {
		if err := os.WriteFile(filepath.Join(a.base, path), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := []byte("preexisting collision")
	if err := os.WriteFile(filepath.Join(a.base, "dest/a.png"), old, 0644); err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(a.base, "doc.md")
	if err := os.WriteFile(docPath, []byte("owned batch fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(docPath)
	if err != nil {
		t.Fatal(err)
	}
	refs := []string{"one/a.png#red", "./one/a.png?x=1#blue", "two/a.png", "dest/stay.png", "https://example.test/a.png", "data:image/png;base64,AA==", "blob:owned"}
	result, err := a.moveImagesTo(d.ID, refs, filepath.Join(a.base, "dest"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 2 || result.Skipped != 3 || result.Token == "" {
		t.Fatalf("bad move result: %+v", result)
	}
	if !strings.HasSuffix(result.References[refs[0]], "#red") || !strings.HasSuffix(result.References[refs[1]], "?x=1#blue") {
		t.Fatal("alias suffix lost")
	}
	if strings.Split(result.References[refs[0]], "#")[0] != strings.Split(result.References[refs[1]], "?")[0] {
		t.Fatal("aliases planned two destinations")
	}
	for _, path := range []string{"one/a.png", "two/a.png"} {
		if _, err := os.Stat(filepath.Join(a.base, path)); !os.IsNotExist(err) {
			t.Fatal("source remains")
		}
	}
	if got, _ := os.ReadFile(filepath.Join(a.base, "dest/a.png")); !bytes.Equal(got, old) {
		t.Fatal("collision overwritten")
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"one/a.png", "two/a.png", "dest/stay.png"} {
		if got, _ := os.ReadFile(filepath.Join(a.base, path)); !bytes.Equal(got, data) {
			t.Fatal("undo failed to restore/preserve file")
		}
	}
	entries, err := os.ReadDir(filepath.Join(a.base, "dest"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("undo left copies: %v %v", entries, err)
	}
	if err := a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	if a.docs[d.ID].Content != "owned batch fixture" {
		t.Fatal("backend changed document")
	}
}

func TestBatchMoveRemovalFailureRollsBackEarlierSourcesAndAllTargets(t *testing.T) {
	a := fixture(t)
	dest := filepath.Join(a.base, "dest")
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	data := clipboardPNG(t)
	for _, name := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(a.base, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	readonly, _ := windows.UTF16PtrFromString(filepath.Join(a.base, "b.png"))
	if err := windows.SetFileAttributes(readonly, windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	defer windows.SetFileAttributes(readonly, windows.FILE_ATTRIBUTE_NORMAL)
	path := filepath.Join(a.base, "doc.md")
	if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.moveImagesTo(d.ID, []string{"a.png", "b.png"}, dest); err == nil {
		t.Fatal("readonly source removal accepted")
	}
	for _, name := range []string{"a.png", "b.png"} {
		if got, _ := os.ReadFile(filepath.Join(a.base, name)); !bytes.Equal(got, data) {
			t.Fatal("failed batch lost source")
		}
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed batch left destination files: %v %v", entries, err)
	}
	if len(a.imageCopies) != 0 {
		t.Fatal("failed batch retained history")
	}
	if _, err := a.moveImagesTo(d.ID, []string{"a.png", "missing.png"}, dest); err == nil {
		t.Fatal("missing source accepted")
	}
	entries, _ = os.ReadDir(dest)
	if len(entries) != 0 {
		t.Fatal("planning failure wrote destination")
	}
}
