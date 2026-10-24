//go:build windows

package edge

import (
	"encoding/json"
	"net/url"
	"strings"
	"unsafe"
)

const markdownReaderDevToolsPrefix = "markdownreader:devtools:"

func markdownReaderOrigin(source string) bool {
	u, err := url.Parse(source)
	return err == nil && u.Scheme == "http" && u.Host == "wails.localhost" && u.User == nil
}

// WebMessageReceived runs on the WebView's COM apartment. Open the inspector
// here rather than calling WebView2 from a Go binding goroutine.
func (e *Chromium) markdownReaderDevTools(message string, sender *ICoreWebView2, args *ICoreWebView2WebMessageReceivedEventArgs) bool {
	if !strings.HasPrefix(message, markdownReaderDevToolsPrefix) {
		return false
	}
	id := strings.TrimPrefix(message, markdownReaderDevToolsPrefix)
	top, topErr := sender.GetSource()
	source, sourceErr := args.GetSource()
	if id == "" || len(id) > 64 || topErr != nil || sourceErr != nil || !markdownReaderOrigin(top) || !markdownReaderOrigin(source) {
		return true
	}
	settings, err := sender.GetSettings()
	if err == nil {
		defer settings.vtbl.Release.Call(uintptr(unsafe.Pointer(settings)))
		err = settings.PutAreDevToolsEnabled(true)
	}
	if err == nil {
		err = sender.OpenDevToolsWindow()
	}
	result := struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Error string `json:"error,omitempty"`
	}{Type: "markdownreader:devtools-result", ID: id}
	if err != nil {
		result.Error = err.Error()
	}
	response, _ := json.Marshal(result)
	if err := sender.PostWebMessageAsString(string(response)); err != nil {
		e.errorCallback(err)
	}
	return true
}
