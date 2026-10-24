package main

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataImageDecodingPreservesBytes(t *testing.T) {
	png := clipboardPNG(t)
	for _, prefix := range []string{"data:image/png;base64,", "DATA:image/png;charset=utf-8;BASE64,"} {
		data, ext, err := decodeDataImage(prefix + base64.StdEncoding.EncodeToString(png))
		if err != nil || ext != ".png" || !bytes.Equal(data, png) {
			t.Fatalf("PNG byte round trip: %s %v", ext, err)
		}
	}
	if data, _, err := decodeDataImage("data:image/png;base64," + url.PathEscape(base64.StdEncoding.EncodeToString(png))); err != nil || !bytes.Equal(data, png) {
		t.Fatal("escaped base64 URI did not preserve image")
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><text>A+B?</text></svg>`
	data, ext, err := decodeDataImage("data:image/svg+xml," + strings.ReplaceAll(svg, " ", "%20"))
	if err != nil || ext != ".svg" || string(data) != svg {
		t.Fatalf("percent data URI corrupted plus sign: %q %v", data, err)
	}
	for _, input := range []string{"data:image/png;base64,invalid!", "data:text/html;base64,QQ==", "data:image/png,", "data:image/png;bad,abc", "data:image/svg+xml,%GG"} {
		if _, _, err := decodeDataImage(input); err == nil {
			t.Fatalf("invalid data accepted: %s", input)
		}
	}
}

func TestDataImageCopyUsesFileHistory(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	png := clipboardPNG(t)
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	target := filepath.Join(a.base, "内嵌.png")
	result, err := a.copyImageTo(d.ID, uri, target, false, "内嵌")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(data, png) {
		t.Fatal("decoded PNG bytes changed")
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("undo retained data URI copy")
	}
	if err := a.SetImageCopyState(d.ID, result.Token, true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, png) {
		t.Fatal("redo did not restore decoded bytes")
	}
	for label, want := range map[string]string{"内嵌": "内嵌.png", "../a": "_.._a.png", "CON": "image_CON.png", "": "image.png"} {
		got := dataImageFilename(label, ".png")
		if label == "../a" {
			if filepath.Base(got) != got || strings.ContainsAny(got, `/\`) {
				t.Fatal("unsafe filename")
			}
			continue
		}
		if got != want {
			t.Fatalf("filename %q: %q", label, got)
		}
	}
}
