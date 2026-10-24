package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentTypesAcrossTreeSearchAndLinks(t *testing.T) {
	a := fixture(t)
	names := []string{"README", "a.mdtxt", "b.mdtext", "c.apib", "d.rmarkdown", "e.rmd", "f.txt", "g.text", "h.MD", "i.markdown", "j.mmd", "k.mkd", "l.mdwn", "m.mdown", "n.mdx"}
	for _, name := range names {
		path := filepath.Join(a.base, name)
		if err := os.WriteFile(path, []byte("# 扩展名验证\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if !isMarkdown(path) {
			t.Errorf("document omitted: %s", name)
		}
		link, err := localLink(path, "标题")
		if err != nil || link.Kind != "markdown" || link.Fragment != "标题" {
			t.Errorf("link %s: %+v %v", name, link, err)
		}
	}
	for _, name := range []string{"script.exe", "archive.zip", "image.png", "config.json"} {
		if isMarkdown(name) {
			t.Errorf("unexpected document: %s", name)
		}
		if err := os.WriteFile(filepath.Join(a.base, name), []byte("扩展名验证"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := a.FolderTree(a.base)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range tree {
		if !entry.Dir {
			count++
		}
	}
	if count != len(names) {
		t.Fatalf("tree files=%d want=%d", count, len(names))
	}
	hits, err := a.SearchFolder(a.base, "扩展名验证")
	if err != nil || len(hits) != len(names) {
		t.Fatalf("search hits=%d want=%d err=%v", len(hits), len(names), err)
	}
}
