package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	win "golang.org/x/sys/windows"
)

func TestPicGoRequestAndResults(t *testing.T) {
	paths := []string{`C:\图片 文件\one.png`, `C:\two&three.png`}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/upload" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("wrong upload request", r.Method, r.URL, r.Header)
		}
		var body struct {
			List []string `json:"list"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !reflect.DeepEqual(body.List, paths) {
			t.Error(body, err)
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []string{"https://example.invalid/one.png", "https://example.invalid/two.png"}})
	}))
	defer server.Close()
	urls, output, err := postPicGoUpload(context.Background(), server.URL+"/upload", paths)
	if err != nil || len(urls) != 2 || !strings.Contains(output, "success") {
		t.Fatalf("%v %s %v", urls, output, err)
	}
}

func TestPicGoRejectsInvalidResponsesAndRedirects(t *testing.T) {
	for _, body := range []string{`{"success":false,"message":"fixture failed"}`, `{"success":true,"result":[]}`, `{"success":true,"result":["javascript:bad"]}`, `{"success":true,"result":["https://example.invalid/a\nhttps://example.invalid/b"]}`, `not JSON`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		if _, _, err := postPicGoUpload(context.Background(), server.URL, []string{"one.png"}); err == nil {
			t.Error("accepted " + body)
		}
		server.Close()
	}
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	if _, _, err := postPicGoUpload(context.Background(), redirect.URL, []string{"one.png"}); err == nil || forwarded {
		t.Fatalf("redirect forwarded=%v err=%v", forwarded, err)
	}
}

func TestPicGoCoreCommandHasSeparateArguments(t *testing.T) {
	program := `C:\Program Files\PicGo 核心\picgo.exe`
	command, err := picGoCoreCommand(program)
	if err != nil {
		t.Fatal(err)
	}
	args, err := win.DecomposeCommandLine(command)
	if err != nil || !reflect.DeepEqual(args, []string{program, "upload"}) {
		t.Fatalf("%v %v", args, err)
	}
	if _, err = picGoCoreCommand(""); err == nil {
		t.Fatal("missing program accepted")
	}
}
