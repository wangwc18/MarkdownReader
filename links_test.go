package main

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLinkTargets(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "中文 空格.md")
	if err := os.WriteFile(path, []byte("# 标题"), 0600); err != nil {
		t.Fatal(err)
	}
	u := url.URL{Path: "中文 空格.md", Fragment: "标题"}
	target, err := resolveLink(root, u.String())
	if err != nil || target.Kind != "markdown" || target.Path != path || target.Fragment != "标题" {
		t.Fatalf("相对文档: %+v %v", target, err)
	}
	u = url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}
	target, err = resolveLink("", u.String())
	if err != nil || target.Path != path {
		t.Fatalf("file URI: %+v %v", target, err)
	}
	for _, reference := range []string{"javascript:alert(1)", "data:text/html,test", "cmd:whoami", "missing.md"} {
		if _, err = resolveLink(root, reference); err == nil {
			t.Fatalf("未拒绝: %s", reference)
		}
	}
	if _, err = resolveLink("", "relative.md"); err == nil {
		t.Fatal("未保存文档不应猜测相对目录")
	}
	target, err = resolveLink(root, "#标题")
	if err != nil || target.Kind != "anchor" || target.Fragment != "标题" {
		t.Fatalf("锚点: %+v %v", target, err)
	}
	target, err = resolveLink(root, "https://example.com/中文?q=1#test")
	if err != nil || target.Kind != "url" {
		t.Fatalf("网址: %+v %v", target, err)
	}
	if err = os.WriteFile(filepath.Join(root, "app.exe"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = resolveLink(root, "app.exe"); err == nil {
		t.Fatal("不应自动运行程序")
	}
}
