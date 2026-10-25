package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBulkHTTPImagesDedupQueriesAndHistory(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	png := clipboardPNG(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer server.Close()
	dir := filepath.Join(a.base, "copies")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	one := server.URL + "/photo.png?token=one"
	two := server.URL + "/photo.png?token=two"
	result, err := a.copyImagesTo(d.ID, []string{one, one + "#view", two}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || result.Copied != 2 || result.Skipped != 0 {
		t.Fatalf("requests=%d result=%+v", requests, result)
	}
	if result.References[one]+"#view" != result.References[one+"#view"] || result.References[one] == result.References[two] {
		t.Fatal(result.References)
	}
	for _, ref := range result.References {
		if strings.Contains(ref, "token=") {
			t.Fatal("HTTP query leaked into local reference", ref)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal(entries)
	}
	for _, entry := range entries {
		got, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil || string(got) != string(png) {
			t.Fatal("changed bytes", err)
		}
	}
	if err = a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("bulk undo retained files")
	}
	if err = a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("bulk redo lost files")
	}
}

func TestBulkHTTPFailureLeavesDirectoryUnchanged(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	png := clipboardPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.png" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer server.Close()
	dir := filepath.Join(a.base, "copies")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.copyImagesTo(d.ID, []string{server.URL + "/photo.png", server.URL + "/missing.png"}, dir); err == nil {
		t.Fatal("accepted missing HTTP image")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed preflight wrote partial files")
	}
}
