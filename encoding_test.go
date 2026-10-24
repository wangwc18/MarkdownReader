package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllOfferedEncodingsRoundTrip(t *testing.T) {
	samples := map[string]string{
		"UTF-8": "中文😀", "UTF-8 BOM": "中文😀", "UTF-16 LE": "中文😀", "UTF-16 BE": "中文😀", "UTF-16 LE (无 BOM)": "中文😀", "UTF-16 BE (无 BOM)": "中文😀",
		"windows-1252": "Café € — résumé", "windows-1251": "Привет мир", "ISO-8859-5": "Привет мир", "IBM866": "Привет мир", "IBM855": "Привет мир", "KOI8-R": "Привет мир", "MacCyrillic": "Привет мир",
		"windows-1250": "Příliš žluťoučký", "ISO-8859-2": "Příliš žluťoučký", "windows-1253": "Καλημέρα", "ISO-8859-7": "Καλημέρα", "windows-1255": "שלום", "ISO-8859-8": "שלום",
		"GB2312": "中文测试", "GBK": "中文测试", "GB18030": "中文😀𠀀", "Big5": "繁體中文", "SHIFT_JIS": "日本語", "EUC-JP": "日本語", "EUC-KR": "한국어", "TIS-620": "ภาษาไทย",
	}
	for _, option := range encodingOptions {
		t.Run(option.Name, func(t *testing.T) {
			sample, ok := samples[option.Name]
			if !ok {
				t.Fatal("新增编码缺少实际字符样本")
			}
			text := "# " + sample + "\r\n内容:"
			if option.Name == "Big5" {
				text = "# " + sample + "\r\n內容:"
			}
			if !strings.HasPrefix(option.Name, "UTF-") && !strings.HasPrefix(option.Name, "GB") && option.Name != "Big5" {
				text = sample + "\r\n"
			}
			d := &Document{Content: strings.ReplaceAll(text, "\r\n", "\n"), Encoding: option.Name, EOL: "CRLF"}
			b, err := encodeDocument(d, false)
			if err != nil {
				t.Fatal(err)
			}
			decoded, name, err := decodeFileAs(b, option.Name)
			if err != nil || decoded != text || name != option.Name {
				t.Fatalf("%q %s %v", decoded, name, err)
			}
		})
	}
}
func TestLegacySaveFailurePreservesOriginal(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "中文 GBK.md")
	d := &Document{Content: "中文\n", Encoding: "GBK", EOL: "LF"}
	original, err := encodeDocument(d, false)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, original, 0600)
	opened, err := a.OpenPathEncoding(path, "GBK")
	if err != nil {
		t.Fatal(err)
	}
	a.UpdateDocument(opened.ID, "中文😀\n", "LF", 1)
	if err = a.saveTo(a.docs[opened.ID], path, true); err == nil {
		t.Fatal("GBK 无法表达 emoji，应阻止保存")
	}
	b, _ := os.ReadFile(path)
	if !bytes.Equal(b, original) {
		t.Fatal("编码失败损坏原文件")
	}
	if _, err = a.SetDocumentEncoding(opened.ID, "UTF-8"); err != nil {
		t.Fatal(err)
	}
	if err = a.saveTo(a.docs[opened.ID], path, true); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if string(b) != "中文😀\n" {
		t.Fatal("转换 UTF-8 后内容不完整")
	}
}
func TestEncodingReopenIsReadOnlyAndRejectsDirtyWithoutConsent(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "编码选择.md")
	original := []byte("# Café\r\n")
	os.WriteFile(path, original, 0600)
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := a.ReopenEncoding(d.ID, "windows-1252", false)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Dirty || !strings.Contains(reopened.Content, "CafÃ©") {
		t.Fatalf("显式编码未生效: %+v", reopened)
	}
	if err = a.saveTo(a.docs[d.ID], path, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !bytes.Equal(b, original) {
		t.Fatal("仅重新打开不应改写文件")
	}
	a.UpdateDocument(d.ID, "未保存修改", "LF", reopened.Revision+1)
	if _, err = a.ReopenEncoding(d.ID, "UTF-8", false); err == nil {
		t.Fatal("重新打开未要求确认丢弃编辑")
	}
	if _, err = a.ReopenEncoding(d.ID, "UTF-8", true); err != nil {
		t.Fatal(err)
	}
	if a.docs[d.ID].Content != "# Café\n" {
		t.Fatal("确认后未重读磁盘原文")
	}
}
func TestAutoDetectionAndInvalidByteProtection(t *testing.T) {
	b, _ := encodeDocument(&Document{Content: "# 中文\n", Encoding: "GB18030", EOL: "LF"}, false)
	text, enc, err := decodeFile(b)
	if err != nil || text != "# 中文\n" || enc != "GB18030" {
		t.Fatalf("中文自动识别: %q %s %v", text, enc, err)
	}
	for _, name := range []string{"UTF-16 LE (无 BOM)", "UTF-16 BE (无 BOM)"} {
		b, _ = encodeDocument(&Document{Content: "# English\n", Encoding: name, EOL: "LF"}, false)
		text, enc, err = decodeFile(b)
		if err != nil || text != "# English\n" || enc != name {
			t.Fatalf("无 BOM 自动识别: %s %q %s %v", name, text, enc, err)
		}
	}
	for _, sample := range []struct {
		bytes []byte
		name  string
	}{{[]byte{0x81}, "GBK"}, {[]byte{0x81, 0x30, 0x81}, "GB18030"}, {[]byte{0x82}, "SHIFT_JIS"}, {[]byte{0, 0, 0, 0}, "auto"}, {[]byte{0x81, 0x40}, "GB2312"}, {[]byte{0x80}, "TIS-620"}} {
		if _, _, err = decodeFileAs(sample.bytes, sample.name); err == nil {
			t.Fatalf("无效输入未拒绝: %x %s", sample.bytes, sample.name)
		}
	}
	if _, err = encodeDocument(&Document{Content: string([]byte{0xff}), Encoding: "UTF-8"}, false); err == nil {
		t.Fatal("不能写入无效 Unicode")
	}
}
func TestStartupDecodeFailureLeavesOtherDocumentsUsable(t *testing.T) {
	a := fixture(t)
	bad := filepath.Join(a.base, "binary.md")
	good := filepath.Join(a.base, "有效.md")
	os.WriteFile(bad, []byte{0, 0, 0, 0}, 0600)
	os.WriteFile(good, []byte("# 正常\n"), 0600)
	previous := os.Args
	os.Args = []string{"MarkdownReader.exe", bad, good}
	defer func() { os.Args = previous }()
	boot, err := a.Bootstrap()
	if err != nil || len(boot.OpenErrors) != 1 || len(boot.Documents) != 1 || boot.Documents[0].Content != "# 正常\n" {
		t.Fatalf("启动应保留可用窗口与正常文件: %+v %v", boot, err)
	}
}
func TestExplicitEncodingSurvivesCloseRenameAndRestart(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "繁體.md")
	b, err := encodeDocument(&Document{Content: "# 繁體中文\n", Encoding: "Big5", EOL: "LF"}, false)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, b, 0600)
	d, err := a.OpenPathEncoding(path, "Big5")
	if err != nil {
		t.Fatal(err)
	}
	a.CloseDocument(d.ID)
	d, err = a.OpenPath(path)
	if err != nil || d.Encoding != "Big5" || d.Content != "# 繁體中文\n" {
		t.Fatalf("关闭后遗失编码: %+v %v", d, err)
	}
	d, err = a.RenameDocument(d.ID, "重命名.md")
	if err != nil {
		t.Fatal(err)
	}
	other := NewApp(a.base)
	if err = other.initStorage(); err != nil {
		t.Fatal(err)
	}
	reopened, err := other.OpenPath(d.Path)
	if err != nil || reopened.Encoding != "Big5" || reopened.Content != d.Content {
		t.Fatalf("重启后编码丢失: %+v %v", reopened, err)
	}
	settings := other.GetSettings()
	settings.FileEncodings[encodingPathKey(d.Path)] = "changed"
	settings.Recent[0] = "changed"
	if other.GetSettings().FileEncodings[encodingPathKey(d.Path)] != "Big5" || other.GetSettings().Recent[0] == "changed" {
		t.Fatal("设置快照不应暴露内部可变映射")
	}
}
func TestFolderSearchDecodesChineseAndRememberedBig5(t *testing.T) {
	a := fixture(t)
	for _, sample := range []struct{ name, encoding, text string }{{"gbk.md", "GBK", "中文搜索"}, {"big5.md", "Big5", "繁體搜尋"}} {
		b, err := encodeDocument(&Document{Content: sample.text, Encoding: sample.encoding, EOL: "LF"}, false)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(a.base, sample.name)
		os.WriteFile(path, b, 0600)
		if sample.encoding == "Big5" {
			d, err := a.OpenPathEncoding(path, sample.encoding)
			if err != nil {
				t.Fatal(err)
			}
			a.CloseDocument(d.ID)
		}
		hits, err := a.SearchFolder(a.base, sample.text)
		if err != nil || len(hits) != 1 || hits[0].Path != path {
			t.Fatalf("编码搜索失败: %+v %v", hits, err)
		}
	}
}
func TestAutomaticReopenClearsExplicitPreference(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(a.base, "utf8.md")
	os.WriteFile(path, []byte("Café"), 0600)
	d, err := a.OpenPathEncoding(path, "windows-1252")
	if err != nil {
		t.Fatal(err)
	}
	d, err = a.ReopenEncoding(d.ID, "auto", false)
	if err != nil || d.Content != "Café" {
		t.Fatalf("自动重新打开: %+v %v", d, err)
	}
	if _, ok := a.GetSettings().FileEncodings[encodingPathKey(path)]; ok {
		t.Fatal("自动模式应清除显式编码选择")
	}
}
