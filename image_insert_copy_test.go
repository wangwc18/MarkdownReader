package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInsertedImageCollisionDoesNotOverwrite(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "copies")
	if err := os.Mkdir(folder, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("new image bytes")
	source := filepath.Join(base, "image.png")
	if err := os.WriteFile(source, data, 0644); err != nil {
		t.Fatal(err)
	}
	occupied := []string{filepath.Join(folder, "image.png"), filepath.Join(folder, "image-"+digest(data)[:8]+".png")}
	for _, p := range occupied {
		if err := os.WriteFile(p, []byte("keep existing"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	target, err := copyInsertedImage(source, folder)
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join(folder, "image-"+digest(data)[:8]+"-1.png") {
		t.Fatal(target)
	}
	for _, p := range occupied {
		got, err := os.ReadFile(p)
		if err != nil || string(got) != "keep existing" {
			t.Fatalf("overwritten %s %v", p, err)
		}
	}
	again, err := copyInsertedImage(source, folder)
	if err != nil || again != target {
		t.Fatalf("repeated insertion %s %v", again, err)
	}
	got, err := os.ReadFile(source)
	if err != nil || string(got) != string(data) {
		t.Fatal("source changed", err)
	}
}

func TestConcurrentInsertedImagesReuseIdenticalCopy(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "copies")
	if err := os.Mkdir(folder, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "image.png")
	if err := os.WriteFile(source, clipboardPNG(t), 0644); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	paths := make([]string, 8)
	errs := make([]error, 8)
	for i := range paths {
		wg.Add(1)
		go func(i int) { defer wg.Done(); paths[i], errs[i] = copyInsertedImage(source, folder) }(i)
	}
	wg.Wait()
	for i := range paths {
		if errs[i] != nil || paths[i] != filepath.Join(folder, "image.png") {
			t.Fatalf("%d %s %v", i, paths[i], errs[i])
		}
	}
	entries, err := os.ReadDir(folder)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%v %v", entries, err)
	}
}
