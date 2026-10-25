package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAllImagesDeduplicationCollisionsAndHistory(t *testing.T) {
	a := fixture(t)
	root := filepath.Join(a.base, "root")
	target := filepath.Join(a.base, "target")
	for _, path := range []string{filepath.Join(root, "a"), filepath.Join(root, "b"), target} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	first, second := filepath.Join(root, "a", "same.png"), filepath.Join(root, "b", "same.png")
	for path, data := range map[string][]byte{first: []byte("first image"), second: []byte("second image"), filepath.Join(target, "same.png"): []byte("existing target")} {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	d := a.NewDocument()
	content := "---\ntypora-root-url: '" + filepath.ToSlash(root) + "'\n---\n"
	if err := a.UpdateDocument(d.ID, content, "LF", 1); err != nil {
		t.Fatal(err)
	}
	result, err := a.copyImagesTo(d.ID, []string{"a/same.png", "/a/same.png", "a/same.png", "b/same.png", "blob:owned-test-fixture"}, target)
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 2 || result.Skipped != 1 || result.Token == "" || result.References["a/same.png"] != result.References["/a/same.png"] || result.References["a/same.png"] == result.References["b/same.png"] {
		t.Fatalf("bad batch result: %+v", result)
	}
	if b, _ := os.ReadFile(filepath.Join(target, "same.png")); string(b) != "existing target" {
		t.Fatal("collision overwrote existing file")
	}
	op := a.imageCopies[result.Token]
	if len(op.Children) != 2 {
		t.Fatal("batch is not one two-file operation")
	}
	if err := os.WriteFile(op.Children[1].Target, []byte("external change"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err == nil {
		t.Fatal("batch undo accepted external modification")
	}
	if b, err := os.ReadFile(op.Children[0].Target); err != nil || !bytes.Equal(b, op.Children[0].After) {
		t.Fatal("failed preflight partially undid batch")
	}
	if err := os.WriteFile(op.Children[1].Target, op.Children[1].After, 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	for _, child := range op.Children {
		if _, err := os.Stat(child.Target); !os.IsNotExist(err) {
			t.Fatal("batch undo left copy")
		}
	}
	if err := a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	for _, child := range op.Children {
		if b, err := os.ReadFile(child.Target); err != nil || !bytes.Equal(b, child.After) {
			t.Fatal("batch redo changed image")
		}
	}
	if a.docs[d.ID].Content != content {
		t.Fatal("backend batch modified Markdown")
	}
}

func TestCopyAllImagesValidatesBeforeWriting(t *testing.T) {
	a := fixture(t)
	target := filepath.Join(a.base, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(a.base, "source.png")
	if err := os.WriteFile(source, []byte("image"), 0644); err != nil {
		t.Fatal(err)
	}
	d := a.NewDocument()
	if _, err := a.copyImagesTo(d.ID, []string{source, filepath.Join(a.base, "missing.png")}, target); err == nil {
		t.Fatal("missing image accepted")
	}
	files, err := os.ReadDir(target)
	if err != nil || len(files) != 0 {
		t.Fatal("failed batch left partial output")
	}
	noop, err := a.copyImagesTo(d.ID, []string{source}, a.base)
	if err != nil || noop.Copied != 0 || noop.Token != "" {
		t.Fatalf("same-directory copy: %+v %v", noop, err)
	}
}

func TestBatchFileHistoryRollsBackAfterWriteFailure(t *testing.T) {
	base := t.TempDir()
	op := &imageCopyOperation{Children: []*imageCopyOperation{
		{Target: filepath.Join(base, "first.png"), After: []byte("first")},
		{Target: filepath.Join(base, "missing-directory", "second.png"), After: []byte("second")},
	}}
	if err := changeImageCopyState(op, true); err == nil {
		t.Fatal("missing parent did not fail")
	}
	if _, err := os.Stat(op.Children[0].Target); !os.IsNotExist(err) {
		t.Fatal("batch failure left previously written file")
	}
	if op.Applied || op.Children[0].Applied {
		t.Fatal("failed batch retained applied state")
	}
}

func TestCopiedImagePathMatchesNativeUnicode(t *testing.T) {
	if got := copiedImagePath(`子目录\透明 色块#1%.svg`); got != "子目录/透明%20色块%231%25.svg" {
		t.Fatalf("native copied image path: %s", got)
	}
}
