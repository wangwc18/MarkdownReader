package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameDocumentPreservesUnsavedContent(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "原文.md")
	original := []byte("![图](assets/图.png)\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.UpdateDocument(d.ID, "未保存的修改\n", "LF", 1); err != nil {
		t.Fatal(err)
	}
	renamed, err := a.RenameDocument(d.ID, "中文 空格.md")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Content != "未保存的修改\n" || !renamed.Dirty || renamed.Path != filepath.Join(a.base, "中文 空格.md") {
		t.Fatalf("编辑状态丢失: %+v", renamed)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("旧名称仍存在")
	}
	b, err := os.ReadFile(renamed.Path)
	if err != nil || !bytes.Equal(b, original) {
		t.Fatalf("重命名不应自动保存编辑内容: %s %v", b, err)
	}
	if a.settings.Recent[0] != renamed.Path {
		t.Fatal("最近文件未更新")
	}
	if _, err = a.RenameDocument(d.ID, "中文 空格.MD"); err != nil {
		t.Fatalf("大小写重命名失败: %v", err)
	}
}
func TestRenameRejectsCollisionAndExternalModification(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "原文.md")
	target := filepath.Join(a.base, "已存在.md")
	os.WriteFile(path, []byte("original"), 0600)
	os.WriteFile(target, []byte("other"), 0600)
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.RenameDocument(d.ID, "已存在.md"); err == nil {
		t.Fatal("未拒绝覆盖")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "other" {
		t.Fatal("覆盖了原有目标")
	}
	for _, name := range []string{"../逃逸.md", "CON.md", "LPT1.txt", "坏?.md", "尾点.md.", "空白.md "} {
		if _, err = a.RenameDocument(d.ID, name); err == nil {
			t.Fatalf("未拒绝名称: %s", name)
		}
	}
	os.WriteFile(path, []byte("external"), 0600)
	if _, err = a.RenameDocument(d.ID, "新文.md"); err == nil {
		t.Fatal("未拦截外部修改")
	}
}
func TestDeleteAndRestoreKeepOriginalBytesAndUnsavedBuffer(t *testing.T) {
	a := fixture(t)
	a.recycle = func(path string) error { return os.Remove(path) }
	path := filepath.Join(a.base, "删除测试.md")
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("# 中文\r\n")...)
	os.WriteFile(path, original, 0600)
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.UpdateDocument(d.ID, "尚未保存的内容\n", "LF", 1)
	deleted, err := a.DeleteDocumentFile(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Path != "" || !deleted.Dirty || !deleted.Detached || deleted.Content != "尚未保存的内容\n" {
		t.Fatalf("编辑缓冲丢失: %+v", deleted)
	}
	a.UpdateDocument(d.ID, "再次修改\n", "LF", 2)
	if !a.docs[d.ID].Dirty {
		t.Fatal("删除后的缓冲应保持未保存")
	}
	list, err := a.ListDeletedFiles()
	if err != nil || len(list) != 1 {
		t.Fatalf("恢复记录: %v %v", list, err)
	}
	os.WriteFile(path, []byte("new occupant"), 0600)
	if _, err = a.RestoreDeletedFile(list[0].ID); err == nil {
		t.Fatal("恢复覆盖了同名文件")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "new occupant" {
		t.Fatal("原路径的新文件被损坏")
	}
	os.Remove(path)
	restored, err := a.RestoreDeletedFile(list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(b, original) {
		t.Fatalf("恢复字节不同: %x %v", b, err)
	}
	if restored.Path != path || a.docs[d.ID].Content != "再次修改\n" {
		t.Fatal("恢复覆盖了未保存缓冲")
	}
	if _, err = a.RestoreDeletedFile(list[0].ID); err == nil {
		t.Fatal("重复恢复应拒绝")
	}
	if err = a.saveTo(a.docs[d.ID], filepath.Join(a.base, "另存.md"), false); err != nil || a.docs[d.ID].Detached || a.docs[d.ID].Dirty {
		t.Fatalf("另存后的状态: %v", err)
	}
}
func TestDeleteFailureAndCorruptBackupDoNotLoseFiles(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "保留.md")
	os.WriteFile(path, []byte("keep"), 0600)
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	a.recycle = func(string) error { return errors.New("test failure") }
	if _, err = a.DeleteDocumentFile(d.ID); err == nil {
		t.Fatal("系统失败未上报")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "keep" || a.docs[d.ID].Path != path {
		t.Fatal("失败影响了原文")
	}
	if list, _ := a.ListDeletedFiles(); len(list) != 0 {
		t.Fatal("未完成的删除不应显示为可恢复记录")
	}
	a.recycle = func(path string) error { return os.Remove(path) }
	if _, err = a.DeleteDocumentFile(d.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := a.ListDeletedFiles()
	if len(list) != 1 {
		t.Fatal("失败删除留下了重复恢复记录")
	}
	dir, _ := a.deletionPath(list[0].ID)
	os.WriteFile(filepath.Join(dir, "content"), []byte("corrupt"), 0600)
	if _, err = a.RestoreDeletedFile(list[0].ID); err == nil {
		t.Fatal("损坏副本不应恢复")
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("写入了损坏内容")
	}
}
func TestClearRecoveryPreservesLiveInstances(t *testing.T) {
	a := fixture(t)
	d := a.NewDocument()
	a.UpdateDocument(d.ID, "当前草稿", "LF", 1)
	live := filepath.Join(a.dataDir, "recovery", "other-live.json")
	stale := filepath.Join(a.dataDir, "recovery", "stale.json")
	b, _ := json.Marshal(RecoveryFile{PID: uint32(os.Getpid()), Documents: []*Document{{Content: "另一窗口草稿", Dirty: true}}})
	os.WriteFile(live, b, 0600)
	os.WriteFile(stale, []byte("[]"), 0600)
	if err := a.ClearRecovery(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatal("删除了活动窗口的恢复文件")
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("未删除过时恢复文件")
	}
	if _, err := os.Stat(filepath.Join(a.dataDir, "recovery", a.instance+".json")); err != nil {
		t.Fatal("当前草稿未重新写入")
	}
}
func TestRecycleFileNative(t *testing.T) {
	if os.Getenv("MARKDOWNREADER_TEST_RECYCLE") != "1" {
		t.Skip("仅显式运行 Windows 回收站集成测试")
	}
	path := filepath.Join(t.TempDir(), "MarkdownReader-回收站集成测试.md")
	if err := os.WriteFile(path, []byte("owned integration fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := recycleFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("系统回收后原路径仍存在")
	}
}
