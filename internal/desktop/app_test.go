package desktop

import (
	"encoding/binary"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUTF16RoundTrip(t *testing.T) {
	for _, enc := range []string{"UTF-16 LE", "UTF-16 BE"} {
		d := &Document{Content: "# 中文😀\n段落\n", Encoding: enc, EOL: "CRLF"}
		b, e := encodeDocument(d, false)
		if e != nil {
			t.Fatal(e)
		}
		text, encoding, e := decodeFile(b)
		if e != nil || encoding != enc || text != "# 中文😀\r\n段落\r\n" {
			t.Fatalf("%s 编码往返失败: %q %v", enc, text, e)
		}
	}
	b := []byte{0xff, 0xfe, 0, 0}
	binary.LittleEndian.PutUint16(b[2:], 0xd800)
	if _, _, e := decodeFile(b); e == nil {
		t.Fatal("未拒绝不完整代理对")
	}
}
func TestDiagramFenceReplacement(t *testing.T) {
	input := "# 标题\n\n```go\nfmt.Println(1)\n```\n\n```mermaid\ngraph LR\nA-->B\n```\n"
	actual := replaceDiagramFences(input, map[string][]string{"mermaid": {"diagram.png"}})
	if !strings.Contains(actual, "![图表](diagram.png)") || !strings.Contains(actual, "```go\nfmt.Println(1)") {
		t.Fatal("图表替换损坏代码块")
	}
}

func fixture(t *testing.T) *App {
	t.Helper()
	a := NewApp(t.TempDir())
	if e := a.initStorage(); e != nil {
		t.Fatal(e)
	}
	return a
}
func TestOpenSaveRoundTripAndConflict(t *testing.T) {
	a := fixture(t)
	p := filepath.Join(a.base, "中文 文档.md")
	original := []byte("\xef\xbb\xbf# 中文\r\n\r\n- [ ] 事项\r\n")
	if e := os.WriteFile(p, original, 0644); e != nil {
		t.Fatal(e)
	}
	d, e := a.OpenPath(p)
	if e != nil {
		t.Fatal(e)
	}
	if d.Encoding != "UTF-8 BOM" || d.EOL != "CRLF" {
		t.Fatalf("编码/换行丢失: %+v", d)
	}
	if e = a.saveTo(a.docs[d.ID], p, true); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if string(b) != string(original) {
		t.Fatalf("未编辑保存改变原文: %q", b)
	}
	external := []byte("外部修改")
	os.WriteFile(p, external, 0644)
	a.docs[d.ID].Content = "本地修改"
	e = a.saveTo(a.docs[d.ID], p, true)
	if e == nil || !strings.Contains(e.Error(), "EXTERNAL_CHANGE") {
		t.Fatal("未阻止外部修改覆盖")
	}
	b, _ = os.ReadFile(p)
	if string(b) != string(external) {
		t.Fatal("外部内容被覆盖")
	}
}
func TestSaveFailureLeavesOriginal(t *testing.T) {
	a := fixture(t)
	p := filepath.Join(a.base, "原文.md")
	os.WriteFile(p, []byte("原文"), 0644)
	if e := atomicWrite(filepath.Join(a.base, "不存在", "文档.md"), []byte("新文")); e == nil {
		t.Fatal("应返回错误")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "原文" {
		t.Fatal("原文件损坏")
	}
}
func TestUndoToSavedContentClearsDirty(t *testing.T) {
	a := fixture(t)
	p := filepath.Join(a.base, "undo.md")
	os.WriteFile(p, []byte("原始内容\n"), 0644)
	d, e := a.OpenPath(p)
	if e != nil {
		t.Fatal(e)
	}
	a.UpdateDocument(d.ID, "改动\n", "LF", 1)
	if !a.docs[d.ID].Dirty {
		t.Fatal("未标记修改")
	}
	a.UpdateDocument(d.ID, "原始内容\n", "LF", 2)
	if a.docs[d.ID].Dirty {
		t.Fatal("恢复原始内容后仍标记修改")
	}
}
func TestResolveSiblingMedia(t *testing.T) {
	a := fixture(t)
	docs := filepath.Join(a.base, "docs")
	os.MkdirAll(docs, 0755)
	p := filepath.Join(docs, "文档.md")
	os.WriteFile(p, []byte("![图](../图片.png)"), 0644)
	os.WriteFile(filepath.Join(a.base, "图片.png"), []byte("image"), 0644)
	d, e := a.OpenPath(p)
	if e != nil {
		t.Fatal(e)
	}
	media, e := a.ResolveMedia(d.ID, "../图片.png")
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", media, nil))
	if w.Code != 200 {
		t.Fatalf("已引用的兄弟目录图片无法读取: %d", w.Code)
	}
	fragment, e := a.ResolveMedia(d.ID, "../图片.png#中文视图")
	if e != nil || !strings.HasSuffix(fragment, "#%E4%B8%AD%E6%96%87%E8%A7%86%E5%9B%BE") {
		t.Fatalf("图片片段应保留且不能成为文件名: %s %v", fragment, e)
	}
}

func TestSaveAsUsesNewMediaRootWithoutChangingMarkdown(t *testing.T) {
	a := fixture(t)
	oldDir, newDir := filepath.Join(a.base, "原目录"), filepath.Join(a.base, "新目录")
	for _, dir := range []string{oldDir, newDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	text := "# 原文\n\n![图片](image.png)\n"
	oldPath := filepath.Join(oldDir, "原文.md")
	if err := os.WriteFile(oldPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	for dir, value := range map[string]string{oldDir: "old image", newDir: "new image"} {
		if err := os.WriteFile(filepath.Join(dir, "image.png"), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := a.OpenPath(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	oldBase := d.BaseURL
	newPath := filepath.Join(newDir, "另存.md")
	if err = a.saveTo(a.docs[d.ID], newPath, false); err != nil {
		t.Fatal(err)
	}
	if a.docs[d.ID].BaseURL == oldBase {
		t.Fatal("save as retained old media root")
	}
	for _, path := range []string{oldPath, newPath} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != text {
			t.Fatalf("Markdown changed: %q %v", data, err)
		}
	}
	media, err := a.ResolveMedia(d.ID, "image.png")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", media, nil))
	if w.Code != 200 || w.Body.String() != "new image" {
		t.Fatalf("wrong media after save as: %d %s", w.Code, w.Body.String())
	}
}
func TestRecoveryRejectsStaleUpdate(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	if e := a.UpdateDocument(d.ID, "最新内容", "LF", 2); e != nil {
		t.Fatal(e)
	}
	a.UpdateDocument(d.ID, "过时内容", "LF", 1)
	if a.docs[d.ID].Content != "最新内容" {
		t.Fatal("过时更新覆盖新内容")
	}
	b, _ := os.ReadFile(filepath.Join(a.dataDir, "recovery", a.instance+".json"))
	if !strings.Contains(string(b), "最新内容") {
		t.Fatal("恢复快照缺失")
	}
	a.CloseDocument(d.ID)
	if _, e := os.Stat(filepath.Join(a.dataDir, "recovery", a.instance+".json")); !os.IsNotExist(e) {
		t.Fatal("已关闭文档草稿未清理")
	}
}
func TestLocalMediaCannotEscapeRoot(t *testing.T) {
	a := fixture(t)
	dir := filepath.Join(a.base, "document")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(a.base, "secret.png"), []byte("secret"), 0644)
	a.mu.Lock()
	url := a.registerRoot(dir)
	a.mu.Unlock()
	r := httptest.NewRequest("GET", url+"../secret.png", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("越界资源状态=%d", w.Code)
	}
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("image"), 0644)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", url+"image.png", nil))
	if w.Code != 200 || w.Body.String() != "image" {
		t.Fatalf("正常图片无法读取: %d %q", w.Code, w.Body.String())
	}
}
func TestSearchUnicodeAndHiddenDirectories(t *testing.T) {
	a := fixture(t)
	os.WriteFile(filepath.Join(a.base, "测试.md"), []byte("第一行\n目标中文\n"), 0644)
	os.Mkdir(filepath.Join(a.base, ".hidden"), 0755)
	os.WriteFile(filepath.Join(a.base, ".hidden", "秘密.md"), []byte("目标中文"), 0644)
	hits, e := a.SearchFolder(a.base, "目标")
	if e != nil || len(hits) != 1 || hits[0].Line != 2 {
		t.Fatalf("搜索结果错误: %+v %v", hits, e)
	}
}
