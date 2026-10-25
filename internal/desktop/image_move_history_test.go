package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileHistoryRestoresSourceAndProtectsExternalFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "源.png")
	target := filepath.Join(root, "目标.png")
	data := []byte("owned image bytes")
	if err := os.WriteFile(source, data, 0644); err != nil {
		t.Fatal(err)
	}
	op := &imageCopyOperation{Children: []*imageCopyOperation{
		{Target: target, After: data},
		{Target: source, Before: data, Existed: true, RemoveAfter: true},
	}}
	if err := changeImageCopyState(op, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source remains: %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("target: %q %v", got, err)
	}
	external := []byte("external replacement")
	if err := os.WriteFile(source, external, 0644); err != nil {
		t.Fatal(err)
	}
	if err := changeImageCopyState(op, false); err == nil {
		t.Fatal("undo overwrote externally created source")
	}
	if got, _ := os.ReadFile(source); !bytes.Equal(got, external) {
		t.Fatal("external source changed")
	}
	if got, _ := os.ReadFile(target); !bytes.Equal(got, data) {
		t.Fatal("preflight failure removed destination")
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := changeImageCopyState(op, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(source); !bytes.Equal(got, data) {
		t.Fatal("undo source not restored")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("undo destination remains: %v", err)
	}
	if err := changeImageCopyState(op, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, external, 0644); err != nil {
		t.Fatal(err)
	}
	if err := changeImageCopyState(op, false); err == nil {
		t.Fatal("undo accepted modified destination")
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("failed undo partially restored source")
	}
}

func TestMoveUndoRestorationFailureKeepsOnlyDiskCopy(t *testing.T) {
	root := t.TempDir()
	missingParent := filepath.Join(root, "missing")
	target := filepath.Join(root, "target.png")
	data := []byte("only disk copy")
	if err := os.WriteFile(target, data, 0644); err != nil {
		t.Fatal(err)
	}
	op := &imageCopyOperation{Applied: true, Children: []*imageCopyOperation{
		{Target: target, After: data, Applied: true},
		{Target: filepath.Join(missingParent, "source.png"), Before: data, Existed: true, RemoveAfter: true, Applied: true},
	}}
	if err := changeImageCopyState(op, false); err == nil {
		t.Fatal("missing source directory accepted")
	}
	if got, _ := os.ReadFile(target); !bytes.Equal(got, data) {
		t.Fatal("undo deleted only disk copy before restoring source")
	}
	if !op.Applied || !op.Children[0].Applied || !op.Children[1].Applied {
		t.Fatal("failed undo advanced operation state")
	}
}
