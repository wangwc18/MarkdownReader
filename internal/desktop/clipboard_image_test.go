package desktop

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/bmp"
)

func TestClipboardRasterFormatsKeepBytesAndExtension(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "formats.md")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	picture.SetNRGBA(1, 1, color.NRGBA{R: 220, G: 10, B: 50, A: 255})
	encoders := map[string]func(*bytes.Buffer) error{
		".png": func(b *bytes.Buffer) error { return png.Encode(b, picture) },
		".jpg": func(b *bytes.Buffer) error { return jpeg.Encode(b, picture, nil) },
		".gif": func(b *bytes.Buffer) error { return gif.Encode(b, picture, nil) },
		".bmp": func(b *bytes.Buffer) error { return bmp.Encode(b, picture) },
	}
	for extension, encode := range encoders {
		t.Run(extension, func(t *testing.T) {
			var buffer bytes.Buffer
			if err := encode(&buffer); err != nil {
				t.Fatal(err)
			}
			reference, err := a.ImportClipboardImage(d.ID, base64.StdEncoding.EncodeToString(buffer.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Ext(reference) != extension {
				t.Fatalf("wrong extension: %s", reference)
			}
			stored, err := os.ReadFile(filepath.Join(a.base, filepath.FromSlash(reference)))
			if err != nil || !bytes.Equal(stored, buffer.Bytes()) {
				t.Fatal("format bytes changed")
			}
		})
	}
}

func clipboardPNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	picture.SetNRGBA(0, 0, color.NRGBA{R: 210, G: 20, B: 30, A: 128})
	var output bytes.Buffer
	if err := png.Encode(&output, picture); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestClipboardImageStoresOriginalAndDeduplicates(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "中文 文档.md")
	if err := os.WriteFile(path, []byte("# 截图\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.settings.ImageFolder = "中文 图片"
	picture := clipboardPNG(t)
	encoded := base64.StdEncoding.EncodeToString(picture)
	first, err := a.ImportClipboardImage(d.ID, encoded)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ImportClipboardImage(d.ID, encoded)
	if err != nil || first != second {
		t.Fatalf("dedup: %q %q %v", first, second, err)
	}
	decoded, err := url.PathUnescape(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(a.base, filepath.FromSlash(decoded)))
	if err != nil || !bytes.Equal(b, picture) {
		t.Fatal("image bytes changed")
	}
	if a.docs[d.ID].Content != "# 截图\n" {
		t.Fatal("storage mutated document")
	}
	if err := os.WriteFile(filepath.Join(a.base, filepath.FromSlash(decoded)), []byte("existing file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ImportClipboardImage(d.ID, encoded); err == nil {
		t.Fatal("collision overwritten")
	}
	b, _ = os.ReadFile(filepath.Join(a.base, filepath.FromSlash(decoded)))
	if string(b) != "existing file" {
		t.Fatal("collision damaged file")
	}
}

func TestClipboardImageRejectsInvalidInputAndUnsafeFolder(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	encoded := base64.StdEncoding.EncodeToString(clipboardPNG(t))
	if _, err := a.ImportClipboardImage(d.ID, encoded); err == nil {
		t.Fatal("unsaved document accepted")
	}
	path := filepath.Join(a.base, "document.md")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"not base64", base64.StdEncoding.EncodeToString([]byte("not an image")), base64.StdEncoding.EncodeToString(clipboardPNG(t)[:33])} {
		if _, err := a.ImportClipboardImage(d.ID, input); err == nil {
			t.Fatal("invalid data accepted")
		}
	}
	for _, folder := range []string{"../outside", `C:\outside`} {
		a.settings.ImageFolder = folder
		if _, err := a.ImportClipboardImage(d.ID, encoded); err == nil {
			t.Fatalf("unsafe folder accepted: %s", folder)
		}
	}
}
