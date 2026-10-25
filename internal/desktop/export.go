package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"html"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var formats = map[string]string{"docx": "docx", "odt": "odt", "rtf": "rtf", "epub": "epub3", "tex": "latex", "wiki": "mediawiki", "rst": "rst", "textile": "textile", "opml": "opml"}

func (a *App) pandoc() string { return filepath.Join(a.base, "tools", "pandoc.exe") }
func (a *App) ImportDialog() (*Document, error) {
	p, e := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "导入文档", Filters: []runtime.FileFilter{{DisplayName: "文档", Pattern: "*.docx;*.odt;*.html;*.htm;*.epub;*.rst;*.tex;*.rtf;*.textile;*.opml"}}})
	if e != nil || p == "" {
		return nil, e
	}
	return a.ImportPath(p)
}
func (a *App) ExportDialog(id, format, html string, repeat bool) (string, error) {
	if _, ok := formats[format]; !ok && format != "html" && format != "html-plain" && format != "pdf" && format != "png" && !strings.HasPrefix(format, "custom:") {
		return "", errors.New("未知导出格式")
	}
	a.mu.Lock()
	d, ok := a.docs[id]
	if !ok {
		a.mu.Unlock()
		return "", errors.New("文档不存在")
	}
	copy := *d
	s := a.settings
	a.lastExportOutput = ""
	a.mu.Unlock()
	preset, err := customExport(s, format)
	if err != nil {
		return "", err
	}
	if preset != nil && preset.Kind == "program" && preset.OutputMode == "none" {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		message, err := a.runExportProgram(ctx, copy, *preset, "", "")
		a.mu.Lock()
		a.lastExportOutput = message
		if err == nil {
			a.settings.LastExport = format
			a.settings.LastExportPath = ""
			err = a.persistSettings()
		}
		a.mu.Unlock()
		return "", err
	}
	writer := formats[format]
	ext := format
	label := strings.ToUpper(format)
	if preset != nil {
		ext = preset.Extension
		writer = preset.Writer
		label = preset.Name
	}
	if ext == "html-plain" {
		ext = "html"
	}
	name := strings.TrimSuffix(copy.Name, filepath.Ext(copy.Name)) + "." + ext
	p := ""
	if repeat && s.LastExport == format {
		p = s.LastExportPath
	}
	if p == "" {
		var e error
		if preset != nil && preset.Kind == "program" && preset.OutputMode == "folder" {
			p, e = runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择导出文件夹：" + label, DefaultDirectory: s.ExportFolder, CanCreateDirectories: true})
		} else {
			p, e = runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出 " + label, DefaultDirectory: s.ExportFolder, DefaultFilename: name, Filters: []runtime.FileFilter{{DisplayName: label, Pattern: "*." + ext}}})
		}
		if e != nil || p == "" {
			return "", e
		}
	}
	var b []byte
	var e error
	switch format {
	case "html", "html-plain":
		b = []byte(html)
	case "pdf", "png":
		b, e = a.renderExport(format)
	default:
		if preset != nil && preset.Kind == "program" {
			var message string
			if preset.OutputMode == "folder" {
				message, e = a.programFolder(copy, *preset, p)
			} else {
				b, message, e = a.programBytes(copy, *preset, p)
			}
			a.mu.Lock()
			a.lastExportOutput = message
			a.mu.Unlock()
			break
		}
		b, e = a.pandocBytes(copy, writer, preset, p)
	}
	if e != nil {
		return "", e
	}
	if preset == nil || preset.Kind != "program" || preset.OutputMode != "folder" {
		if e = atomicWrite(p, b); e != nil {
			return "", e
		}
	}
	a.mu.Lock()
	a.settings.LastExport = format
	a.settings.LastExportPath = p
	e = a.persistSettings()
	a.mu.Unlock()
	if e != nil {
		return p, e
	}
	if folder := exportAfterDirectory(s, preset, p); folder != "" {
		if err := exec.Command("explorer.exe", folder).Start(); err != nil {
			return p, fmt.Errorf("已导出，但无法打开输出目录：%w", err)
		}
	}
	return p, nil
}

func exportAfterDirectory(settings Settings, preset *ExportPreset, output string) string {
	open := settings.OpenAfterExport
	if preset != nil {
		if preset.Kind == "program" && preset.OutputMode == "none" {
			return ""
		}
		switch preset.AfterExport {
		case "none":
			open = false
		case "open-folder":
			open = true
		}
	}
	if !open || output == "" {
		return ""
	}
	if preset != nil && preset.Kind == "program" && preset.OutputMode == "folder" {
		return filepath.Clean(output)
	}
	return filepath.Dir(output)
}

// Finish validation and cleanup before ExportDialog replaces the destination.
// A converter can write auxiliary files, so all of its outputs share a bound
// temporary root rather than using recursive cleanup in the document folder.
func (a *App) pandocBytes(doc Document, writer string, preset *ExportPreset, destination string) (data []byte, resultErr error) {
	forbidden := []string{a.base, filepath.Dir(destination)}
	if doc.Path != "" {
		forbidden = append(forbidden, filepath.Dir(doc.Path))
	}
	stage, err := newExportStage(forbidden...)
	if err != nil {
		return nil, err
	}
	defer stage.Close()
	defer func() {
		if err := stage.Cleanup(); err != nil {
			data = nil
			resultErr = errors.Join(resultErr, err)
		}
	}()
	mediaDir := filepath.Join(stage.Path, "media")
	if err := stage.root.Mkdir("media", 0700); err != nil {
		return nil, err
	}
	content := doc.Content
	if writer == "docx" || writer == "odt" || writer == "epub3" || writer == "epub" || writer == "rtf" {
		content, err = a.captureDiagrams(doc.Content, mediaDir)
		if err != nil {
			return nil, err
		}
	}
	resourcePath := mediaDir
	if doc.Path != "" {
		resourcePath += string(os.PathListSeparator) + filepath.Dir(doc.Path)
	}
	var extra []string
	if preset != nil {
		extra, err = exportArguments(preset.Arguments, doc, destination)
		if err != nil {
			return nil, err
		}
	}
	output := "output" + filepath.Ext(destination)
	if err := a.convertWithPandoc(content, doc.Path, writer, resourcePath, filepath.Join(stage.Path, output), extra); err != nil {
		return nil, err
	}
	if err := stage.Verify(); err != nil {
		return nil, err
	}
	return stage.root.ReadFile(output)
}

type cdpTarget struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	WebSocket string `json:"webSocketDebuggerUrl"`
}
type cdpClient struct {
	ws  *websocket.Conn
	seq int
}

func (c *cdpClient) call(method string, params interface{}, out interface{}) error {
	c.seq++
	id := c.seq
	c.ws.SetWriteDeadline(time.Now().Add(20 * time.Second))
	if e := c.ws.WriteJSON(map[string]interface{}{"id": id, "method": method, "params": params}); e != nil {
		return e
	}
	c.ws.SetReadDeadline(time.Now().Add(45 * time.Second))
	for {
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if e := c.ws.ReadJSON(&msg); e != nil {
			return e
		}
		if msg.ID != id {
			continue
		}
		if msg.Error != nil {
			return errors.New(msg.Error.Message)
		}
		if out != nil {
			return json.Unmarshal(msg.Result, out)
		}
		return nil
	}
}
func (a *App) connectCDP() (*cdpClient, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	r, e := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", a.debugPort))
	if e != nil {
		return nil, fmt.Errorf("无法连接本地渲染引擎：%w", e)
	}
	defer r.Body.Close()
	var targets []cdpTarget
	if e = json.NewDecoder(r.Body).Decode(&targets); e != nil {
		return nil, e
	}
	var target *cdpTarget
	for i := range targets {
		if targets[i].Type == "page" && strings.Contains(targets[i].URL, "wails") {
			target = &targets[i]
			break
		}
	}
	if target == nil {
		for i := range targets {
			if targets[i].Type == "page" {
				target = &targets[i]
				break
			}
		}
	}
	if target == nil {
		return nil, errors.New("找不到编辑器渲染页面")
	}
	ws, _, e := websocket.DefaultDialer.Dial(target.WebSocket, nil)
	if e != nil {
		return nil, e
	}
	return &cdpClient{ws: ws}, nil
}
func (a *App) renderExport(format string) ([]byte, error) {
	cdp, e := a.connectCDP()
	if e != nil {
		return nil, e
	}
	defer cdp.ws.Close()
	var result struct {
		Data string `json:"data"`
	}
	if format == "pdf" {
		s := a.GetSettings().PDF
		template := func(text string) string {
			text = html.EscapeString(text)
			text = strings.ReplaceAll(text, "{page}", "<span class=\"pageNumber\"></span>")
			text = strings.ReplaceAll(text, "{pages}", "<span class=\"totalPages\"></span>")
			text = strings.ReplaceAll(text, "{title}", "<span class=\"title\"></span>")
			text = strings.ReplaceAll(text, "{date}", "<span class=\"date\"></span>")
			return "<div style=\"width:100%;font-size:9px;text-align:center;\">" + text + "</div>"
		}
		e = cdp.call("Page.printToPDF", map[string]interface{}{"printBackground": true, "preferCSSPageSize": true, "paperWidth": s.Width / 25.4, "paperHeight": s.Height / 25.4, "marginTop": s.Margin / 25.4, "marginBottom": s.Margin / 25.4, "marginLeft": s.Margin / 25.4, "marginRight": s.Margin / 25.4, "displayHeaderFooter": s.Header != "" || s.Footer != "", "headerTemplate": template(s.Header), "footerTemplate": template(s.Footer)}, &result)
	} else {
		var layout struct {
			Result struct {
				Value struct {
					X      float64 `json:"x"`
					Y      float64 `json:"y"`
					Width  float64 `json:"width"`
					Height float64 `json:"height"`
				} `json:"value"`
			} `json:"result"`
		}
		e = cdp.call("Runtime.evaluate", map[string]interface{}{"expression": "(()=>{const r=document.querySelector('#print-area').getBoundingClientRect();return {x:r.x+scrollX,y:r.y+scrollY,width:r.width,height:r.height}})()", "returnByValue": true}, &layout)
		if e == nil {
			v := layout.Result.Value
			if v.Width < 1 || v.Height < 1 {
				return nil, errors.New("导出区域为空")
			}
			e = cdp.call("Page.captureScreenshot", map[string]interface{}{"format": "png", "captureBeyondViewport": true, "clip": map[string]interface{}{"x": v.X, "y": v.Y, "width": v.Width, "height": v.Height, "scale": 1}}, &result)
		}
	}
	if e != nil {
		return nil, e
	}
	return base64.StdEncoding.DecodeString(result.Data)
}

type diagramRect struct {
	Language string  `json:"language"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}

func (a *App) captureDiagrams(markdown, dir string) (string, error) {
	cdp, e := a.connectCDP()
	if e != nil {
		return "", e
	}
	defer cdp.ws.Close()
	var result struct {
		Result struct {
			Value []diagramRect `json:"value"`
		} `json:"result"`
	}
	script := `Array.from(document.querySelectorAll('#print-area [class*="language-"]')).filter(el=>el.querySelector('svg')).map(el=>{const r=el.getBoundingClientRect();return {language:el.className.match(/language-([\w.-]+)/)?.[1],x:r.x+scrollX,y:r.y+scrollY,width:r.width,height:r.height}})`
	if e = cdp.call("Runtime.evaluate", map[string]interface{}{"expression": script, "returnByValue": true}, &result); e != nil {
		return "", e
	}
	images := map[string][]string{}
	for i, r := range result.Result.Value {
		if r.Width < 1 || r.Height < 1 {
			continue
		}
		var screenshot struct {
			Data string `json:"data"`
		}
		if e = cdp.call("Page.captureScreenshot", map[string]interface{}{"format": "png", "captureBeyondViewport": true, "clip": map[string]interface{}{"x": r.X, "y": r.Y, "width": r.Width, "height": r.Height, "scale": 2}}, &screenshot); e != nil {
			return "", e
		}
		b, e := base64.StdEncoding.DecodeString(screenshot.Data)
		if e != nil {
			return "", e
		}
		name := fmt.Sprintf("mr-diagram-%d.png", i)
		if e = os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			return "", e
		}
		images[r.Language] = append(images[r.Language], name)
	}
	return replaceDiagramFences(markdown, images), nil
}
func replaceDiagramFences(markdown string, images map[string][]string) string {
	lines := strings.SplitAfter(markdown, "\n")
	var out strings.Builder
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
			out.WriteString(lines[i])
			continue
		}
		char := line[0]
		count := 0
		for count < len(line) && line[count] == char {
			count++
		}
		if count < 3 {
			out.WriteString(lines[i])
			continue
		}
		lang := strings.TrimSpace(line[count:])
		names := images[lang]
		if len(names) == 0 {
			out.WriteString(lines[i])
			continue
		}
		end := i + 1
		for end < len(lines) {
			closeLine := strings.TrimSpace(lines[end])
			if len(closeLine) >= count && strings.Trim(closeLine, string(char)) == "" {
				break
			}
			end++
		}
		if end == len(lines) {
			out.WriteString(lines[i])
			continue
		}
		out.WriteString("![图表](" + names[0] + ")\n")
		images[lang] = names[1:]
		i = end
	}
	return out.String()
}
