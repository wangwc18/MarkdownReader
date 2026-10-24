package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRemoteImageInsertionModesAndDisabledNoDownload(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "doc.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	png := clipboardPNG(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/missing" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer server.Close()
	reference := server.URL + "/photo.png?sample=1#片段"
	a.settings.ImageInsertAction = "copy"
	got, err := a.PrepareImageReference(d.ID, reference, "测试")
	if err != nil || got != reference || requests.Load() != 0 {
		t.Fatalf("disabled %q %v requests=%d", got, err, requests.Load())
	}
	a.settings.ApplyRemoteImageRules = true
	a.settings.ApplyLocalImageRules = false
	got, err = a.PrepareImageReference(d.ID, reference, "测试")
	if err != nil || got != "assets/photo.png#%E7%89%87%E6%AE%B5" {
		t.Fatalf("copy %q %v", got, err)
	}
	bytes, err := os.ReadFile(filepath.Join(a.base, "assets", "photo.png"))
	if err != nil || string(bytes) != string(png) {
		t.Fatal("copy bytes", err)
	}
	t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
	exe, _ := os.Executable()
	a.settings.ImageInsertAction = "upload"
	a.settings.ImageUploader = "custom"
	a.settings.ImageUploadCommand = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- success`, exe)
	got, err = a.PrepareImageReference(d.ID, reference, "测试")
	if err != nil || got != "https://example.invalid/image-0.png" {
		t.Fatalf("upload %q %v", got, err)
	}
	if _, err = a.PrepareImageReference(d.ID, server.URL+"/missing", "测试"); err == nil {
		t.Fatal("404 accepted")
	}
	if a.docs[d.ID].Content != "original" {
		t.Fatal("backend rewrote document")
	}
	left, _ := filepath.Glob(filepath.Join(a.dataDir, "image-network-insert-*"))
	if len(left) != 0 {
		t.Fatal("temporary files left", left)
	}
	a.settings.ImageInsertAction = "none"
	before := requests.Load()
	got, err = a.PrepareImageReference(d.ID, reference, "测试")
	if err != nil || got != reference || requests.Load() != before {
		t.Fatal("none downloaded", err)
	}
}

func TestRemoteImageInsertionChecksDocumentAfterDownload(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "doc.md")
	os.WriteFile(path, []byte("original"), 0644)
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.settings.ImageInsertAction = "copy"
	a.settings.ApplyRemoteImageRules = true
	ready, release := make(chan struct{}), make(chan struct{})
	var released sync.Once
	png := clipboardPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(ready)
		<-release
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer server.Close()
	defer released.Do(func() { close(release) })
	finished := make(chan error, 1)
	go func() { _, err := a.PrepareImageReference(d.ID, server.URL+"/photo.png", "测试"); finished <- err }()
	<-ready
	a.mu.Lock()
	a.docs[d.ID].Content = "edited"
	a.mu.Unlock()
	released.Do(func() { close(release) })
	if err = <-finished; err == nil || !strings.Contains(err.Error(), "文档已变化") {
		t.Fatalf("%v", err)
	}
	if _, err = os.Stat(filepath.Join(a.base, "assets")); !os.IsNotExist(err) {
		t.Fatal("stale asset created", err)
	}
}
