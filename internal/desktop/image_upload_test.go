package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImageUploadProcess(t *testing.T) {
	if os.Getenv("MARKDOWNREADER_UPLOAD_FIXTURE") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	if args[0] == "wait" {
		if err := os.WriteFile(args[1], []byte("ready"), 0600); err != nil {
			os.Exit(9)
		}
		until := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(args[2]); err == nil {
				break
			}
			if time.Now().After(until) {
				os.Exit(10)
			}
			time.Sleep(10 * time.Millisecond)
		}
		args = append([]string{"success"}, args[3:]...)
	}
	if args[0] == "fail" {
		fmt.Fprintln(os.Stderr, "local fixture failed")
		os.Exit(7)
	}
	if args[0] == "bad" {
		fmt.Println("javascript:invalid")
		os.Exit(0)
	}
	if args[0] == "args" {
		fmt.Println(strings.Join(args[1:], "|"))
		os.Exit(0)
	}
	fmt.Println("Upload Success:")
	for i := range args[1:] {
		fmt.Printf("https://example.invalid/image-%d.png\n", i)
	}
	os.Exit(0)
}

func TestImageUploadProtocolAndArguments(t *testing.T) {
	for _, valid := range []string{"https://example.invalid/a.png", "http://localhost/a", "file:///C:/image.png"} {
		if _, err := uploadResultURLs("log\n"+valid+"\r\n", 1); err != nil {
			t.Fatal(err)
		}
	}
	for _, invalid := range []string{"javascript:bad", "data:image/png;base64,AA==", "https:///no-host", "log\n"} {
		if _, err := uploadResultURLs(invalid, 1); err == nil {
			t.Fatal("accepted " + invalid)
		}
	}
	if _, err := uploadResultURLs("https://example.invalid/one", 2); err == nil {
		t.Fatal("missing result accepted")
	}
	t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	doc := Document{Path: filepath.Join(base, "中文 文档.md")}
	command := fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- args "${filename}" "${filepath}"`, exe)
	_, output, err := runImageUpload(context.Background(), base, doc, command, []string{`C:\images\one & two.png`})
	if err == nil {
		t.Fatal("non-URL output accepted")
	}
	want := "中文 文档.md|" + doc.Path + `|C:\images\one & two.png`
	if strings.TrimSpace(output) != want {
		t.Fatalf("%q != %q", output, want)
	}
	command = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- fail`, exe)
	if _, output, err = runImageUpload(context.Background(), base, doc, command, []string{"a"}); err == nil || !strings.Contains(output, "local fixture failed") {
		t.Fatalf("%q %v", output, err)
	}
}

func TestImageUploadLocalDedupAndOriginalContent(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "doc.md")
	text := "![one](image.png)\n![two](./image.png)"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(a.base, "image.png")
	data := clipboardPNG(t)
	if err := os.WriteFile(image, data, 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
	exe, _ := os.Executable()
	a.settings.ImageUploader = "custom"
	a.settings.ImageUploadCommand = fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- success`, exe)
	result, err := a.UploadImages(d.ID, []string{"image.png", "./image.png", "https://example.invalid/existing.png"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Uploaded != 1 || result.Skipped != 1 || result.References["image.png"] != result.References["./image.png"] || result.Original != text {
		t.Fatalf("%+v", result)
	}
	if a.docs[d.ID].Content != text {
		t.Fatal("backend mutated document")
	}
	got, err := os.ReadFile(image)
	if err != nil || string(got) != string(data) {
		t.Fatal("source changed", err)
	}
}

func TestImageUploadValidationSamplesAreCleaned(t *testing.T) {
	a := fixture(t)
	t.Setenv("MARKDOWNREADER_UPLOAD_FIXTURE", "1")
	exe, _ := os.Executable()
	command := fmt.Sprintf(`"%s" -test.run=TestImageUploadProcess -- success`, exe)
	result, err := a.TestImageUploader("custom", command)
	if err != nil || result.Uploaded != 2 || len(result.References) != 2 {
		t.Fatalf("%+v %v", result, err)
	}
	paths, err := filepath.Glob(filepath.Join(a.dataDir, "image-upload-test-*"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("samples not cleaned %v %v", paths, err)
	}
	if a.settings.ImageUploader != "" || a.settings.ImageUploadCommand != "" {
		t.Fatal("test saved unconfirmed settings")
	}
}
