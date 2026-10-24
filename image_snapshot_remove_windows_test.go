package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLockedImageRemovalBlocksWritersAndRenames(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "源图.png")
	data := []byte("original owned image")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	locked, err := openImageSnapshotForRemoval(path, data)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	if err := os.WriteFile(path, []byte("external"), 0644); err == nil {
		t.Fatal("writer not blocked while snapshot locked")
	}
	if err := os.Rename(path, filepath.Join(root, "renamed.png")); err == nil {
		t.Fatal("rename not blocked while snapshot locked")
	}
	if err := os.Remove(path); err == nil {
		t.Fatal("path deletion not blocked while snapshot locked")
	}
	if err := locked.Close(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatal("blocked writer changed original bytes")
	}
	if err := removeImageSnapshot(path, data); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("matched snapshot not removed: %v", err)
	}
}

func TestSnapshotRemovalRejectsChangedBytesAndDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "changed.png")
	external := []byte("changed since planning")
	if err := os.WriteFile(path, external, 0644); err != nil {
		t.Fatal(err)
	}
	if err := removeImageSnapshot(path, []byte("old bytes")); err == nil {
		t.Fatal("changed image removed")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, external) {
		t.Fatal("changed file not retained")
	}
	if err := removeImageSnapshot(root, nil); err == nil {
		t.Fatal("directory accepted")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal("directory changed")
	}
}
