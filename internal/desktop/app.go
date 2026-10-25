package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	win "golang.org/x/sys/windows"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

type Document struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	Content       string `json:"content"`
	Hash          string `json:"hash"`
	EOL           string `json:"eol"`
	Encoding      string `json:"encoding"`
	Dirty         bool   `json:"dirty"`
	Detached      bool   `json:"detached"`
	BaseURL       string `json:"baseURL"`
	Revision      int    `json:"revision"`
	ImportID      string `json:"importID"`
	SpellLanguage string `json:"spellLanguage"`
	SavedContent  string `json:"-"`
	SavedEOL      string `json:"-"`
	SavedEncoding string `json:"-"`
}
type Settings struct {
	CustomExports         []ExportPreset    `json:"customExports,omitempty"`
	FileEncodings         map[string]string `json:"fileEncodings,omitempty"`
	Theme                 string            `json:"theme"`
	FontSize              int               `json:"fontSize"`
	AutoSave              bool              `json:"autoSave"`
	SaveOnSwitch          bool              `json:"saveOnSwitch"`
	EOL                   string            `json:"eol"`
	FinalNewline          bool              `json:"finalNewline"`
	Indent                int               `json:"indent"`
	Spellcheck            bool              `json:"spellcheck"`
	SpellLanguage         string            `json:"spellLanguage"`
	PersonalWords         map[string]bool   `json:"personalWords"`
	Pair                  bool              `json:"pair"`
	MarkdownPair          bool              `json:"markdownPair"`
	Focus                 bool              `json:"focus"`
	Typewriter            bool              `json:"typewriter"`
	LineNumbers           bool              `json:"lineNumbers"`
	CodeWrap              bool              `json:"codeWrap"`
	ReadingSpeed          int               `json:"readingSpeed"`
	Recent                []string          `json:"recent"`
	Startup               string            `json:"startup"`
	LastFolder            string            `json:"lastFolder"`
	CopyImages            bool              `json:"copyImages"`
	ImageInsertAction     string            `json:"imageInsertAction"`
	ApplyLocalImageRules  bool              `json:"applyLocalImageRules"`
	ApplyRemoteImageRules bool              `json:"applyRemoteImageRules"`
	ImageFolder           string            `json:"imageFolder"`
	ImageFolderCustom     bool              `json:"imageFolderCustom"`
	RelativeImages        bool              `json:"relativeImages"`
	ImageUploader         string            `json:"imageUploader"`
	ImageUploadCommand    string            `json:"imageUploadCommand"`
	ImageUploaderProgram  string            `json:"imageUploaderProgram"`
	AllowYAMLImageUpload  bool              `json:"allowYAMLImageUpload"`
	Shortcuts             map[string]string `json:"shortcuts"`
	ExportFolder          string            `json:"exportFolder"`
	OpenAfterExport       bool              `json:"openAfterExport"`
	LastExport            string            `json:"lastExport"`
	LastExportPath        string            `json:"lastExportPath"`
	CustomCSS             string            `json:"customCSS"`
	ParagraphIndent       bool              `json:"paragraphIndent"`
	Markdown              MarkdownSettings  `json:"markdown"`
	PDF                   PDFSettings       `json:"pdf"`
}
type MarkdownSettings struct {
	StrictMode           bool   `json:"strictMode"`
	InlineMathDigit      bool   `json:"inlineMathDigit"`
	Physics              bool   `json:"physics"`
	MathLineBreaks       bool   `json:"mathLineBreaks"`
	MathNumbering        bool   `json:"mathNumbering"`
	PlainHTMLMathLatex   bool   `json:"plainHTMLMathLatex"`
	KeepSingleLineBreaks bool   `json:"keepSingleLineBreaks"`
	ShowBRMarkers        bool   `json:"showBRMarkers"`
	Mark                 bool   `json:"mark"`
	Sup                  bool   `json:"sup"`
	Sub                  bool   `json:"sub"`
	HeadingStyle         string `json:"headingStyle"`
	ListMarker           string `json:"listMarker"`
	SmartQuotes          bool   `json:"smartQuotes"`
	SmartDashes          bool   `json:"smartDashes"`
	SmartRender          bool   `json:"smartRender"`
}
type PDFSettings struct {
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Margin  float64 `json:"margin"`
	BreakH1 bool    `json:"breakH1"`
	Header  string  `json:"header"`
	Footer  string  `json:"footer"`
}
type Entry struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Dir      bool    `json:"dir"`
	Children []Entry `json:"children,omitempty"`
}
type SearchHit struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
type Boot struct {
	Settings   Settings    `json:"settings"`
	Documents  []*Document `json:"documents"`
	Recovery   []*Document `json:"recovery"`
	Version    string      `json:"version"`
	Runtime    string      `json:"runtime"`
	OpenErrors []string    `json:"openErrors"`
}
type RecoveryFile struct {
	PID       uint32      `json:"pid"`
	Documents []*Document `json:"documents"`
}

func readRecovery(b []byte) ([]*Document, bool) {
	var file RecoveryFile
	if json.Unmarshal(b, &file) == nil && file.PID != 0 {
		h, e := win.OpenProcess(win.PROCESS_QUERY_LIMITED_INFORMATION, false, file.PID)
		if e == nil {
			defer win.CloseHandle(h)
			var code uint32
			if win.GetExitCodeProcess(h, &code) == nil && code == 259 {
				return nil, true
			}
		} else if e == win.ERROR_ACCESS_DENIED {
			return nil, true
		}
		return file.Documents, false
	}
	var legacy []*Document
	if json.Unmarshal(b, &legacy) == nil {
		return legacy, false
	}
	return nil, false
}

type App struct {
	lastExportOutput        string
	lastImageUploadOutput   string
	ctx                     context.Context
	base, dataDir, instance string
	debugPort               int
	mu                      sync.Mutex
	traceMu                 sync.Mutex
	docs                    map[string]*Document
	roots                   map[string]string
	imageCopies             map[string]*imageCopyOperation
	settings                Settings
	settingsBase            map[string]interface{}
	allowClose              bool
	recycle                 func(string) error
}

func NewApp(base string) *App {
	return &App{base: base, dataDir: filepath.Join(base, "data"), instance: uuid.NewString(), docs: map[string]*Document{}, roots: map[string]string{}, settings: Settings{Theme: "Github", FontSize: 16, EOL: "LF", SpellLanguage: "en-US", Indent: 2, Pair: true, CodeWrap: true, ReadingSpeed: 382, Startup: "new", ImageFolder: "assets", ApplyLocalImageRules: true, RelativeImages: true, Shortcuts: map[string]string{}, Markdown: MarkdownSettings{StrictMode: true, MathLineBreaks: true, ShowBRMarkers: true, KeepSingleLineBreaks: true, Mark: true, Sup: true, Sub: true, HeadingStyle: "atx", ListMarker: "-"}, PDF: PDFSettings{Width: 210, Height: 297, Margin: 15}}}
}
func (a *App) initStorage() error {
	if e := os.MkdirAll(filepath.Join(a.dataDir, "recovery"), 0755); e != nil {
		return e
	}
	p := filepath.Join(a.dataDir, ".write-test-"+a.instance)
	if e := os.WriteFile(p, []byte("ok"), 0600); e != nil {
		return e
	}
	os.Remove(p)
	b, e := os.ReadFile(filepath.Join(a.dataDir, "settings.json"))
	if e == nil {
		if e = json.Unmarshal(b, &a.settings); e != nil {
			return fmt.Errorf("配置文件损坏，原文件保留：%w", e)
		}
	}
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	a.settingsBase, _ = settingsObject(a.settings)
	return nil
}
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func digest(b []byte) string               { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func atomicWrite(path string, b []byte) error {
	d := filepath.Dir(path)
	f, e := os.CreateTemp(d, ".markdownreader-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if s, e := os.Stat(path); e == nil {
		if e = os.Chmod(tmp, s.Mode()); e != nil {
			return e
		}
	}
	from, e := win.UTF16PtrFromString(tmp)
	if e != nil {
		return e
	}
	to, e := win.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	return win.MoveFileEx(from, to, win.MOVEFILE_REPLACE_EXISTING|win.MOVEFILE_WRITE_THROUGH)
}
func (a *App) settingsSnapshot() Settings {
	s := a.settings
	s.CustomExports = cloneExportPresets(s.CustomExports)
	s.Recent = append([]string(nil), s.Recent...)
	s.PersonalWords = map[string]bool{}
	for word, enabled := range a.settings.PersonalWords {
		s.PersonalWords[word] = enabled
	}
	s.Shortcuts = map[string]string{}
	for key, value := range a.settings.Shortcuts {
		s.Shortcuts[key] = value
	}
	s.FileEncodings = map[string]string{}
	for path, name := range a.settings.FileEncodings {
		s.FileEncodings[path] = name
	}
	return s
}
func (a *App) GetSettings() Settings { a.mu.Lock(); defer a.mu.Unlock(); return a.settingsSnapshot() }
func (a *App) SetSettings(s Settings) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.setSettings(s)
}
func (a *App) setSettings(s Settings) error {
	if s.SpellLanguage == "" {
		s.SpellLanguage = "en-US"
	}
	if s.SpellLanguage != "en-US" && s.SpellLanguage != "en-GB" {
		return errors.New("无效的默认拼写语言")
	}
	if err := validatePersonalWords(s.PersonalWords); err != nil {
		return err
	}
	if s.ImageInsertAction != "" && s.ImageInsertAction != "none" && s.ImageInsertAction != "copy" && s.ImageInsertAction != "upload" {
		return errors.New("无效的图片插入规则")
	}
	if s.ImageInsertAction != "" {
		s.CopyImages = s.ImageInsertAction == "copy"
	}
	if err := validateExportPresets(s.CustomExports); err != nil {
		return err
	}
	s.CustomExports = cloneExportPresets(s.CustomExports)
	if s.FontSize < 10 || s.FontSize > 40 {
		return errors.New("字体大小必须在10到40之间")
	}
	if s.Indent < 1 || s.Indent > 8 {
		return errors.New("缩进必须在1到8之间")
	}
	if s.ReadingSpeed < 1 {
		return errors.New("阅读速度必须大于0")
	}
	if s.PDF.Width < 50 || s.PDF.Width > 1000 || s.PDF.Height < 50 || s.PDF.Height > 1000 || s.PDF.Margin < 0 || s.PDF.Margin*2 >= s.PDF.Width || s.PDF.Margin*2 >= s.PDF.Height {
		return errors.New("PDF 页面大小或页边距无效")
	}
	if s.Markdown.ListMarker != "-" && s.Markdown.ListMarker != "+" && s.Markdown.ListMarker != "*" {
		return errors.New("无效的无序列表标记")
	}
	s.Recent = a.settings.Recent
	s.FileEncodings = a.settings.FileEncodings
	previous := a.settingsSnapshot()
	a.settings = s
	if err := a.persistSettings(); err != nil {
		a.settings = previous
		return err
	}
	return nil
}
func (a *App) Bootstrap() (Boot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	boot := Boot{Settings: a.settings, Documents: []*Document{}, Recovery: []*Document{}, Version: "0.1.0", Runtime: "系统 WebView2"}
	if _, e := os.Stat(filepath.Join(a.base, "runtime", "msedgewebview2.exe")); e == nil {
		boot.Runtime = "随包固定版本 WebView2"
	}
	files, _ := filepath.Glob(filepath.Join(a.dataDir, "recovery", "*.json"))
	for _, p := range files {
		if filepath.Base(p) == a.instance+".json" {
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		ds, live := readRecovery(b)
		if !live {
			boot.Recovery = append(boot.Recovery, ds...)
		}
	}
	for _, p := range os.Args[1:] {
		if strings.HasPrefix(p, "-") {
			continue
		}
		d, e := a.open(p)
		if e != nil {
			boot.OpenErrors = append(boot.OpenErrors, p+"："+e.Error())
			continue
		}
		boot.Documents = append(boot.Documents, d)
	}
	if len(boot.Documents) == 0 && a.settings.Startup == "last" {
		for _, p := range a.settings.Recent {
			d, e := a.open(p)
			if e == nil {
				boot.Documents = append(boot.Documents, d)
				break
			}
		}
	}
	boot.Settings = a.settingsSnapshot()
	return boot, nil
}
func (a *App) newDoc() *Document {
	d := &Document{ID: uuid.NewString(), Name: "未命名", EOL: a.settings.EOL, SavedEOL: a.settings.EOL, Encoding: "UTF-8", SavedEncoding: "UTF-8"}
	a.docs[d.ID] = d
	return d
}
func (a *App) NewDocument() *Document { a.mu.Lock(); defer a.mu.Unlock(); return a.newDoc() }
func (a *App) registerRoot(root string) string {
	if canonical, err := filepath.EvalSymlinks(root); err == nil {
		root = canonical
	}
	for id, r := range a.roots {
		if r == root {
			return "/files/" + id + "/"
		}
	}
	id := uuid.NewString()
	a.roots[id] = root
	return "/files/" + id + "/"
}
func decodeUnicodeFile(b []byte) (string, string, error) {
	enc := "UTF-8"
	if len(b) >= 2 && ((b[0] == 0xff && b[1] == 0xfe) || (b[0] == 0xfe && b[1] == 0xff)) {
		if (len(b)-2)%2 != 0 {
			return "", "", errors.New("UTF-16 文件字节数不完整")
		}
		var order binary.ByteOrder = binary.LittleEndian
		enc = "UTF-16 LE"
		if b[0] == 0xfe {
			order = binary.BigEndian
			enc = "UTF-16 BE"
		}
		words := make([]uint16, (len(b)-2)/2)
		for i := range words {
			words[i] = order.Uint16(b[2+i*2:])
		}
		for i := 0; i < len(words); i++ {
			u := words[i]
			if u >= 0xd800 && u <= 0xdbff {
				if i+1 >= len(words) || words[i+1] < 0xdc00 || words[i+1] > 0xdfff {
					return "", "", errors.New("UTF-16 文件包含无效代理对")
				}
				i++
			} else if u >= 0xdc00 && u <= 0xdfff {
				return "", "", errors.New("UTF-16 文件包含无效代理对")
			}
		}
		return string(utf16.Decode(words)), enc, nil
	}
	if len(b) >= 3 && string(b[:3]) == "\xef\xbb\xbf" {
		b = b[3:]
		enc = "UTF-8 BOM"
	}
	if !utf8.Valid(b) {
		return "", "", errors.New("文件不是有效 UTF-8；已拒绝打开以免损坏内容")
	}
	return string(b), enc, nil
}
func (a *App) open(path string) (*Document, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	for _, d := range a.docs {
		if strings.EqualFold(d.Path, p) {
			copy := *d
			return &copy, nil
		}
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	name := a.settings.FileEncodings[encodingPathKey(p)]
	if name == "" {
		name = "auto"
	}
	text, enc, e := decodeFileAs(b, name)
	if e != nil {
		return nil, e
	}
	eol := "LF"
	if strings.Contains(text, "\r\n") {
		eol = "CRLF"
	}
	d := &Document{ID: uuid.NewString(), Path: p, Name: filepath.Base(p), Content: strings.ReplaceAll(text, "\r\n", "\n"), Hash: digest(b), EOL: eol, Encoding: enc, BaseURL: a.registerRoot(filepath.Dir(p))}
	a.docs[d.ID] = d
	d.SavedContent = d.Content
	d.SavedEOL = d.EOL
	d.SavedEncoding = d.Encoding
	a.addRecent(p)
	copy := *d
	return &copy, nil
}
func (a *App) addRecent(p string) {
	recent := []string{p}
	for _, v := range a.settings.Recent {
		if !strings.EqualFold(v, p) && len(recent) < 30 {
			recent = append(recent, v)
		}
	}
	a.settings.Recent = recent
	a.persistSettings()
}
func (a *App) OpenPath(path string) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.open(path)
}
func (a *App) OpenDialog() (*Document, error) {
	p, e := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "打开 Markdown 文档", Filters: []runtime.FileFilter{{DisplayName: "Markdown / 文本", Pattern: markdownFilePattern}, {DisplayName: "所有文件", Pattern: "*"}}})
	if e != nil || p == "" {
		return nil, e
	}
	return a.OpenPath(p)
}
func (a *App) UpdateDocument(id, content, eol string, revision int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return errors.New("文档不存在")
	}
	if revision < d.Revision {
		return nil
	}
	d.Revision = revision
	if d.Content != content || d.EOL != eol {
		d.Content = content
		d.EOL = eol
		d.Dirty = d.Detached || content != d.SavedContent || eol != d.SavedEOL || d.Encoding != d.SavedEncoding
	}
	return a.writeRecovery()
}
func (a *App) writeRecovery() error {
	ds := []*Document{}
	for _, d := range a.docs {
		if d.Dirty {
			ds = append(ds, d)
		}
	}
	p := filepath.Join(a.dataDir, "recovery", a.instance+".json")
	if len(ds) == 0 {
		if e := os.Remove(p); e != nil && !os.IsNotExist(e) {
			return e
		}
		return nil
	}
	b, e := json.Marshal(RecoveryFile{PID: uint32(os.Getpid()), Documents: ds})
	if e != nil {
		return e
	}
	return atomicWrite(p, b)
}
func encodeUnicodeDocument(d *Document, finalNewline bool) []byte {
	text := strings.ReplaceAll(d.Content, "\r\n", "\n")
	if finalNewline && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if d.EOL == "CRLF" {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if d.Encoding == "UTF-8 BOM" {
		text = "\xef\xbb\xbf" + text
	}
	if strings.HasPrefix(d.Encoding, "UTF-16 LE") || strings.HasPrefix(d.Encoding, "UTF-16 BE") {
		var order binary.ByteOrder = binary.LittleEndian
		b := []byte{0xff, 0xfe}
		if strings.HasPrefix(d.Encoding, "UTF-16 BE") {
			order = binary.BigEndian
			b = []byte{0xfe, 0xff}
		}
		if strings.HasSuffix(d.Encoding, "(无 BOM)") {
			b = nil
		}
		for _, u := range utf16.Encode([]rune(text)) {
			var word [2]byte
			order.PutUint16(word[:], u)
			b = append(b, word[:]...)
		}
		return b
	}
	return []byte(text)
}
func (a *App) SaveDocument(id string, saveAs bool) (*Document, error) {
	a.mu.Lock()
	d, ok := a.docs[id]
	if !ok {
		a.mu.Unlock()
		return nil, errors.New("文档不存在")
	}
	copy := *d
	a.mu.Unlock()
	p := copy.Path
	if saveAs || p == "" {
		var e error
		p, e = runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "保存 Markdown 文档", DefaultFilename: copy.Name + func() string {
			if copy.Path == "" && filepath.Ext(copy.Name) == "" {
				return ".md"
			}
			return ""
		}(), Filters: []runtime.FileFilter{{DisplayName: "Markdown", Pattern: "*.md"}}})
		if e != nil || p == "" {
			return nil, e
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d = a.docs[id]
	if e := a.saveTo(d, p, copy.Path != "" && strings.EqualFold(p, copy.Path)); e != nil {
		return nil, e
	}
	result := *d
	return &result, a.writeRecovery()
}
func (a *App) saveTo(d *Document, p string, check bool) error {
	if check {
		b, e := os.ReadFile(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if os.IsNotExist(e) || digest(b) != d.Hash {
			return errors.New("EXTERNAL_CHANGE:文件已被外部修改或删除；请重新加载或另存为，避免覆盖")
		}
		if !d.Dirty && !a.settings.FinalNewline {
			return nil
		}
	}
	b, err := encodeDocument(d, a.settings.FinalNewline)
	if err != nil {
		return err
	}
	if d.ImportID != "" {
		if err := a.copyImportedMedia(d.ImportID, filepath.Dir(p)); err != nil {
			return err
		}
	}
	if e := atomicWrite(p, b); e != nil {
		return e
	}
	d.Path = p
	d.Name = filepath.Base(p)
	d.Hash = digest(b)
	d.Dirty = false
	d.Detached = false
	d.ImportID = ""
	d.SavedContent = d.Content
	d.SavedEOL = d.EOL
	d.SavedEncoding = d.Encoding
	d.BaseURL = a.registerRoot(filepath.Dir(p))
	a.rememberEncoding(p, d.Encoding)
	a.addRecent(p)
	return nil
}
func (a *App) CheckExternal(id string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok || d.Path == "" {
		return "", nil
	}
	b, e := os.ReadFile(d.Path)
	if os.IsNotExist(e) {
		return "deleted", nil
	}
	if e != nil {
		return "", e
	}
	if digest(b) != d.Hash {
		return "changed", nil
	}
	return "", nil
}
func (a *App) ReloadDocument(id string) (*Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}
	b, e := os.ReadFile(d.Path)
	if e != nil {
		return nil, e
	}
	text, enc, e := decodeFileAs(b, d.Encoding)
	if e != nil {
		return nil, e
	}
	d.Content = strings.ReplaceAll(text, "\r\n", "\n")
	d.Encoding = enc
	d.Hash = digest(b)
	d.Dirty = false
	if strings.Contains(text, "\r\n") {
		d.EOL = "CRLF"
	} else {
		d.EOL = "LF"
	}
	d.SavedContent = d.Content
	d.SavedEOL = d.EOL
	d.SavedEncoding = d.Encoding
	a.writeRecovery()
	copy := *d
	return &copy, nil
}
func (a *App) CloseDocument(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.docs, id)
	for token, operation := range a.imageCopies {
		if operation.DocumentID == id {
			delete(a.imageCopies, token)
		}
	}
	return a.writeRecovery()
}
func (a *App) RestoreDocument(source Document) *Document {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := source
	d.ID = uuid.NewString()
	d.Dirty = true
	if d.Path != "" {
		d.BaseURL = a.registerRoot(filepath.Dir(d.Path))
	} else if d.ImportID != "" {
		if root, err := a.importRoot(d.ImportID); err == nil {
			d.BaseURL = a.registerRoot(root)
		}
	}
	a.docs[d.ID] = &d
	a.writeRecovery()
	return &d
}
func (a *App) ClearRecovery() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	files, _ := filepath.Glob(filepath.Join(a.dataDir, "recovery", "*.json"))
	for _, p := range files {
		if filepath.Base(p) != a.instance+".json" {
			b, e := os.ReadFile(p)
			if e != nil {
				continue
			}
			if _, live := readRecovery(b); live {
				continue
			}
		}
		if e := os.Remove(p); e != nil {
			return e
		}
	}
	return a.writeRecovery()
}
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	a.traceLifecycle("before-close", map[string]any{"allowed": a.allowClose})
	if !a.allowClose {
		a.mu.Unlock()
		runtime.EventsEmit(ctx, "close-request")
		return true
	}
	a.allowClose = false
	dirty := false
	for _, d := range a.docs {
		if d.Dirty {
			dirty = true
		}
	}
	a.mu.Unlock()
	if !dirty {
		return false
	}
	choice, e := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: "未保存的更改", Message: "关闭前保存所有文档？", Buttons: []string{"保存", "不保存", "取消"}, DefaultButton: "保存", CancelButton: "取消"})
	if e != nil || choice == "取消" || choice == "" {
		return true
	}
	if choice == "保存" {
		a.mu.Lock()
		ids := []string{}
		for id, d := range a.docs {
			if d.Dirty {
				ids = append(ids, id)
			}
		}
		a.mu.Unlock()
		for _, id := range ids {
			d, e := a.SaveDocument(id, false)
			if e != nil {
				runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.ErrorDialog, Message: e.Error()})
				return true
			}
			if d == nil {
				return true
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	os.Remove(filepath.Join(a.dataDir, "recovery", a.instance+".json"))
	return false
}
func (a *App) RequestClose() {
	a.traceLifecycle("request-close", nil)
	a.mu.Lock()
	a.allowClose = true
	a.mu.Unlock()
	go runtime.Quit(a.ctx)
}
func (a *App) OpenFolderDialog() (string, error) {
	p, e := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "打开文件夹"})
	if p != "" && e == nil {
		a.mu.Lock()
		a.settings.LastFolder = p
		a.persistSettings()
		a.registerRoot(p)
		a.mu.Unlock()
	}
	return p, e
}
func scanTree(root string, depth int) ([]Entry, error) {
	if depth > 24 {
		return nil, nil
	}
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil, e
	}
	out := []Entry{}
	for _, v := range entries {
		if strings.HasPrefix(v.Name(), ".") || v.Name() == "node_modules" {
			continue
		}
		p := filepath.Join(root, v.Name())
		if v.Type()&os.ModeSymlink != 0 {
			continue
		}
		if v.IsDir() {
			children, _ := scanTree(p, depth+1)
			out = append(out, Entry{Name: v.Name(), Path: p, Dir: true, Children: children})
		} else if isMarkdown(v.Name()) {
			out = append(out, Entry{Name: v.Name(), Path: p})
		}
		if len(out) > 5000 {
			break
		}
	}
	return out, nil
}
func (a *App) FolderTree(path string) ([]Entry, error) { return scanTree(path, 0) }
func (a *App) SearchFolder(root, query string) ([]SearchHit, error) {
	hits := []SearchHit{}
	encodings := a.GetSettings().FileEncodings
	if query == "" {
		return hits, nil
	}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !isMarkdown(p) {
			return nil
		}
		info, e := d.Info()
		if e != nil || info.Size() > 16*1024*1024 {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil
		}
		name := encodings[encodingPathKey(p)]
		if name == "" {
			name = "auto"
		}
		text, _, err := decodeFileAs(b, name)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(text, "\n") {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				hits = append(hits, SearchHit{p, i + 1, strings.TrimSpace(line)})
				if len(hits) >= 1000 {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	return hits, e
}
func (a *App) NewWindow(path string) error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	args := []string{}
	if path != "" {
		args = append(args, path)
	}
	return exec.Command(exe, args...).Start()
}
func (a *App) Reveal(path string) error {
	return exec.Command("explorer.exe", "/select,", path).Start()
}
func (a *App) Properties(path string) (map[string]interface{}, error) {
	s, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	return map[string]interface{}{"path": path, "size": s.Size(), "modified": s.ModTime().Format("2006-01-02 15:04:05")}, nil
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/files/") {
		http.NotFound(w, r)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/files/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	a.mu.Lock()
	root, ok := a.roots[parts[0]]
	a.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(root, filepath.FromSlash(parts[1]))
	real, e := filepath.EvalSymlinks(p)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	rel, e := filepath.Rel(root, real)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		http.Error(w, "路径超出文档目录", 403)
		return
	}
	ext := strings.ToLower(filepath.Ext(real))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico", ".mp3", ".mp4", ".ogg", ".wav":
	default:
		http.Error(w, "仅允许文档媒体资源", 403)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, real)
}

type InsertedImage struct {
	Reference string `json:"reference"`
	Label     string `json:"label"`
}

func (a *App) InsertImage(id string) (*InsertedImage, error) {
	p, e := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "插入图片", Filters: []runtime.FileFilter{{DisplayName: "图像", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.svg;*.webp;*.bmp"}}})
	if e != nil || p == "" {
		return nil, e
	}
	reference, e := a.insertLocalImage(id, p)
	if e != nil {
		return nil, e
	}
	return &InsertedImage{Reference: reference, Label: strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))}, nil
}

func (a *App) insertLocalImage(id, p string) (string, error) {
	a.mu.Lock()
	locked := true
	defer func() {
		if locked {
			a.mu.Unlock()
		}
	}()
	d, ok := a.docs[id]
	if !ok {
		return "", errors.New("文档不存在")
	}
	if d.Path == "" {
		return "", errors.New("请先保存文档，再插入本地图片")
	}
	action, e := imageInsertionAction(d.Content, a.settings)
	if e != nil {
		return "", e
	}
	if action == "upload" {
		snapshot, settings := *d, a.settings
		a.mu.Unlock()
		locked = false
		return a.uploadInsertedImage(snapshot, settings, p)
	}
	root := filepath.Dir(d.Path)
	referenceRoot, _, e := documentImageRoot(d.Content, root)
	if e != nil {
		return "", e
	}
	if action == "copy" {
		folder, _, e := insertedImageFolder(d.Content, root, a.settings.ImageFolder, d.Path, a.settings.ImageFolderCustom)
		if e != nil {
			return "", e
		}
		p, e = copyInsertedImage(p, folder)
		if e != nil {
			return "", e
		}
	}
	return imageReference(referenceRoot, p, a.settings.RelativeImages)
}
func (a *App) ResolveMedia(id, reference string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.docs[id]
	if !ok {
		return "", errors.New("文档不存在")
	}
	if strings.HasPrefix(reference, "data:") || strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
		return reference, nil
	}
	reference, fragment, _ := strings.Cut(reference, "#")
	reference, query, _ := strings.Cut(reference, "?")
	p, e := url.PathUnescape(reference)
	if e != nil {
		return "", e
	}
	if strings.HasPrefix(p, "file:///") {
		p = strings.TrimPrefix(p, "file:///")
	}
	if !filepath.IsAbs(p) {
		root := ""
		if d.Path != "" {
			root = filepath.Dir(d.Path)
		}
		if d.Path == "" {
			key := strings.Trim(strings.TrimPrefix(d.BaseURL, "/files/"), "/")
			root = a.roots[key]
		}
		p, e = imagePathFromRoot(d.Content, root, p)
		if e != nil {
			return "", e
		}
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return "", e
	}
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico":
	default:
		return "", errors.New("不支持的媒体格式")
	}
	result := a.registerRoot(filepath.Dir(p)) + url.PathEscape(filepath.Base(p))
	if query != "" {
		result += "?" + query
	}
	if fragment != "" {
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			return "", err
		}
		u := url.URL{Fragment: decoded}
		result += "#" + u.EscapedFragment()
	}
	return result, nil
}
func (a *App) ToggleAlwaysOnTop(v bool) { runtime.WindowSetAlwaysOnTop(a.ctx, v) }
func (a *App) ToggleFullscreen(v bool) {
	if v {
		runtime.WindowFullscreen(a.ctx)
	} else {
		runtime.WindowUnfullscreen(a.ctx)
	}
}
func (a *App) SetTitle(title string) { runtime.WindowSetTitle(a.ctx, title+" — MarkdownReader") }
func (a *App) LogError(message string) error {
	f, e := os.OpenFile(filepath.Join(a.dataDir, "errors.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), message)
	return e
}
