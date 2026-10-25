package desktop

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

type EncodingOption struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

var encodingOptions = []EncodingOption{
	{"UTF-8", "UTF-8"}, {"UTF-8 BOM", "UTF-8（BOM）"}, {"UTF-16 LE", "UTF-16 LE"}, {"UTF-16 BE", "UTF-16 BE"},
	{"UTF-16 LE (无 BOM)", "UTF-16 LE（无 BOM）"}, {"UTF-16 BE (无 BOM)", "UTF-16 BE（无 BOM）"},
	{"windows-1252", "Western (windows-1252)"}, {"windows-1251", "Cyrillic (windows-1251)"}, {"ISO-8859-5", "Cyrillic (ISO-8859-5)"},
	{"IBM866", "Cyrillic (IBM866)"}, {"IBM855", "Cyrillic (IBM855)"}, {"KOI8-R", "Cyrillic (KOI8-R)"}, {"MacCyrillic", "Cyrillic (MacCyrillic)"},
	{"windows-1250", "Central European (windows-1250)"}, {"ISO-8859-2", "Central European (ISO-8859-2)"},
	{"windows-1253", "Greek (windows-1253)"}, {"ISO-8859-7", "Greek (ISO-8859-7)"},
	{"windows-1255", "Hebrew (windows-1255)"}, {"ISO-8859-8", "Hebrew (ISO-8859-8)"},
	{"GB2312", "简体中文 (GB2312)"}, {"GBK", "简体中文 (GBK)"}, {"GB18030", "简体中文 (GB18030)"}, {"Big5", "繁体中文 (Big5)"},
	{"SHIFT_JIS", "日本語 (SHIFT_JIS)"}, {"EUC-JP", "日本語 (EUC-JP)"}, {"EUC-KR", "한국어 (EUC-KR)"}, {"TIS-620", "ไทย (TIS-620)"},
}
var legacyEncodings = map[string]encoding.Encoding{
	"windows-1252": charmap.Windows1252, "windows-1251": charmap.Windows1251, "ISO-8859-5": charmap.ISO8859_5,
	"IBM866": charmap.CodePage866, "IBM855": charmap.CodePage855, "KOI8-R": charmap.KOI8R, "MacCyrillic": charmap.MacintoshCyrillic,
	"windows-1250": charmap.Windows1250, "ISO-8859-2": charmap.ISO8859_2, "windows-1253": charmap.Windows1253, "ISO-8859-7": charmap.ISO8859_7,
	"windows-1255": charmap.Windows1255, "ISO-8859-8": charmap.ISO8859_8, "GB2312": simplifiedchinese.GBK, "GBK": simplifiedchinese.GBK,
	"GB18030": simplifiedchinese.GB18030, "Big5": traditionalchinese.Big5, "SHIFT_JIS": japanese.ShiftJIS, "EUC-JP": japanese.EUCJP, "EUC-KR": korean.EUCKR, "TIS-620": charmap.Windows874,
}

func (a *App) Encodings() []EncodingOption { return append([]EncodingOption(nil), encodingOptions...) }
func encodingPathKey(path string) string   { return strings.ToLower(filepath.Clean(path)) }
func (a *App) rememberEncoding(path, name string) {
	if a.settings.FileEncodings == nil {
		a.settings.FileEncodings = map[string]string{}
	}
	key := encodingPathKey(path)
	if strings.EqualFold(name, "auto") {
		delete(a.settings.FileEncodings, key)
	} else {
		a.settings.FileEncodings[key] = name
	}
}
func canonicalEncoding(name string) (string, error) {
	for _, option := range encodingOptions {
		if strings.EqualFold(name, option.Name) {
			return option.Name, nil
		}
	}
	return "", fmt.Errorf("不支持的编码：%s", name)
}
func validateDecoded(text string) error {
	if strings.ContainsRune(text, 0) {
		return errors.New("文件包含 NUL 字符，请选择正确的文本编码；不会按乱码保存")
	}
	return nil
}
func validateLegacyBytes(b []byte, name string) error {
	if name == "GB2312" {
		for i := 0; i < len(b); i++ {
			if b[i] < 0x80 {
				continue
			}
			if i+1 >= len(b) || b[i] < 0xa1 || b[i] > 0xf7 || b[i+1] < 0xa1 || b[i+1] > 0xfe {
				return errors.New("字节不属于 GB2312 编码范围")
			}
			i++
		}
	}
	if name == "TIS-620" {
		for _, c := range b {
			if c >= 0x80 && !(c >= 0xa1 && c <= 0xda || c >= 0xdf && c <= 0xfb) {
				return errors.New("字节不属于 TIS-620 编码范围")
			}
		}
	}
	return nil
}
func decodeLegacy(b []byte, name string) (string, string, error) {
	codec := legacyEncodings[name]
	if codec == nil {
		return "", "", errors.New("不支持的文本编码")
	}
	if err := validateLegacyBytes(b, name); err != nil {
		return "", "", err
	}
	out, err := codec.NewDecoder().Bytes(b)
	if err != nil {
		return "", "", err
	}
	if bytes.Contains(out, []byte("�")) {
		roundTrip, e := codec.NewEncoder().Bytes(out)
		if e != nil || !bytes.Equal(roundTrip, b) {
			return "", "", fmt.Errorf("文件包含无效的 %s 字节；已拒绝替换字符，原文件保留", name)
		}
	}
	text := string(out)
	if err = validateDecoded(text); err != nil {
		return "", "", err
	}
	return text, name, nil
}
func decodeFile(b []byte) (string, string, error) { return decodeFileAs(b, "auto") }
func decodeFileAs(b []byte, name string) (string, string, error) {
	if strings.EqualFold(name, "auto") {
		if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.HasPrefix(b, []byte{0xfe, 0xff}) || bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) || utf8.Valid(b) && !bytes.Contains(b, []byte{0}) {
			text, enc, err := decodeUnicodeFile(b)
			if err == nil {
				err = validateDecoded(text)
			}
			return text, enc, err
		}
		if bytes.Contains(b, []byte{0}) {
			if len(b)%2 == 0 && len(b) > 0 {
				zeros := [2]int{}
				for i, c := range b {
					if c == 0 {
						zeros[i%2]++
					}
				}
				if zeros[1] > len(b)/8 && zeros[1] > zeros[0]*4 {
					return decodeFileAs(b, "UTF-16 LE (无 BOM)")
				}
				if zeros[0] > len(b)/8 && zeros[0] > zeros[1]*4 {
					return decodeFileAs(b, "UTF-16 BE (无 BOM)")
				}
			}
			return "", "", errors.New("无法自动识别含 NUL 的文件；请使用“按编码打开”，原文件未修改")
		}
		text, enc, err := decodeLegacy(b, "GB18030")
		if err != nil {
			return "", "", fmt.Errorf("自动识别失败，请使用“按编码打开”：%w", err)
		}
		return text, enc, nil
	}
	canonical, err := canonicalEncoding(name)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(canonical, "UTF-16") {
		bom := []byte{0xff, 0xfe}
		if strings.HasPrefix(canonical, "UTF-16 BE") {
			bom = []byte{0xfe, 0xff}
		}
		hasBOM := bytes.HasPrefix(b, bom)
		payload := b
		if !hasBOM {
			payload = append(append([]byte{}, bom...), b...)
		}
		text, _, err := decodeUnicodeFile(payload)
		if err == nil {
			err = validateDecoded(text)
		}
		enc := strings.TrimSuffix(canonical, " (无 BOM)")
		if !hasBOM {
			enc += " (无 BOM)"
		}
		return text, enc, err
	}
	if strings.HasPrefix(canonical, "UTF-8") {
		text, enc, err := decodeUnicodeFile(b)
		if err == nil && strings.HasPrefix(enc, "UTF-16") {
			return "", "", errors.New("文件含 UTF-16 BOM，不能按 UTF-8 打开")
		}
		if err == nil {
			err = validateDecoded(text)
		}
		return text, enc, err
	}
	return decodeLegacy(b, canonical)
}
func encodeDocument(d *Document, finalNewline bool) ([]byte, error) {
	if !utf8.ValidString(d.Content) {
		return nil, errors.New("文本含无效 Unicode，未写入文件")
	}
	if err := validateDecoded(d.Content); err != nil {
		return nil, err
	}
	name, err := canonicalEncoding(d.Encoding)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(name, "UTF-") {
		copy := *d
		copy.Encoding = name
		return encodeUnicodeDocument(&copy, finalNewline), nil
	}
	copy := *d
	copy.Encoding = "UTF-8"
	utf := encodeUnicodeDocument(&copy, finalNewline)
	b, err := legacyEncodings[name].NewEncoder().Bytes(utf)
	if err == nil {
		err = validateLegacyBytes(b, name)
	}
	if err != nil {
		return nil, fmt.Errorf("当前文本无法保存为 %s，请选择 UTF-8；原文件未修改：%w", name, err)
	}
	return b, nil
}
func (a *App) SetDocumentEncoding(id, name string) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}
	canonical, err := canonicalEncoding(name)
	if err != nil {
		return nil, err
	}
	copy := *d
	copy.Encoding = canonical
	if _, err = encodeDocument(&copy, a.settings.FinalNewline); err != nil {
		return nil, err
	}
	d.Encoding = canonical
	d.Dirty = d.Detached || d.Content != d.SavedContent || d.EOL != d.SavedEOL || d.Encoding != d.SavedEncoding
	if err = a.writeRecovery(); err != nil {
		return nil, err
	}
	copy = *d
	return &copy, nil
}
func (a *App) reopenEncoding(d *Document, name string) (*Document, error) {
	b, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	text, enc, err := decodeFileAs(b, name)
	if err != nil {
		return nil, err
	}
	d.Content = strings.ReplaceAll(text, "\r\n", "\n")
	d.Encoding = enc
	d.Hash = digest(b)
	d.EOL = "LF"
	if strings.Contains(text, "\r\n") {
		d.EOL = "CRLF"
	}
	d.SavedContent = d.Content
	d.SavedEOL = d.EOL
	d.SavedEncoding = d.Encoding
	d.Dirty = false
	d.Detached = false
	d.Revision++
	a.rememberEncoding(d.Path, name)
	_ = a.persistSettings()
	copy := *d
	return &copy, a.writeRecovery()
}
func (a *App) ReopenEncoding(id, name string, discard bool) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok || d.Path == "" {
		return nil, errors.New("请先打开一个已保存的文件")
	}
	if d.Dirty && !discard {
		return nil, errors.New("文档有未保存修改，重新打开前需要确认")
	}
	return a.reopenEncoding(d, name)
}
func (a *App) OpenPathEncoding(path, name string) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	for _, d := range a.docs {
		if strings.EqualFold(path, d.Path) {
			if d.Dirty {
				return nil, errors.New("文件已打开且有未保存修改，请先保存或确认重新打开")
			}
			return a.reopenEncoding(d, name)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text, enc, err := decodeFileAs(b, name)
	if err != nil {
		return nil, err
	}
	eol := "LF"
	if strings.Contains(text, "\r\n") {
		eol = "CRLF"
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	d := &Document{ID: uuid.NewString(), Path: path, Name: filepath.Base(path), Content: text, Hash: digest(b), EOL: eol, Encoding: enc, SavedContent: text, SavedEOL: eol, SavedEncoding: enc, BaseURL: a.registerRoot(filepath.Dir(path))}
	a.docs[d.ID] = d
	a.rememberEncoding(path, name)
	a.addRecent(path)
	copy := *d
	return &copy, nil
}
func (a *App) OpenDialogEncoding(name string) (*Document, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "按编码打开文档", Filters: []runtime.FileFilter{{DisplayName: "Markdown / 文本", Pattern: markdownFilePattern}, {DisplayName: "所有文件", Pattern: "*"}}})
	if err != nil || path == "" {
		return nil, err
	}
	return a.OpenPathEncoding(path, name)
}
