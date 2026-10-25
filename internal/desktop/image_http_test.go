package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPImageCopyKeepsBytesAndHistory(t *testing.T) {
	a := fixture(t)
	png := clipboardPNG(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/photo.png", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer server.Close()
	d := a.NewDocument()
	source, err := a.imageSourceForCopy(d.ID, server.URL+"/photo.png#fragment", "样本")
	if err != nil || source.Name != "photo.png" {
		t.Fatalf("%+v %v", source, err)
	}
	target := filepath.Join(a.base, "download.png")
	result, err := a.copyImageSourceTo(d.ID, source, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("unexpected repeat request %d", requests)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(png) {
		t.Fatal("download bytes", err)
	}
	if err = a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("copy undo did not remove file")
	}
	if err = a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	if _, err = a.imageSourceForCopy(d.ID, server.URL+"/redirect", "样本"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPImageFailuresDoNotCreateCopy(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.WriteHeader(404)
		case "/oversize":
			w.Header().Set("Content-Length", fmt.Sprint(21<<20))
			w.Header().Set("Content-Type", "image/png")
		case "/empty":
			w.Header().Set("Content-Type", "image/png")
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("not an image"))
		}
	}))
	defer server.Close()
	for _, name := range []string{"status", "oversize", "empty", "html"} {
		target := filepath.Join(a.base, name+".png")
		if _, err := a.copyImageTo(d.ID, server.URL+"/"+name, target, false); err == nil {
			t.Error("accepted " + name)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatal("created failed copy", name)
		}
	}
}
