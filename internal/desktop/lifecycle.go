package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Diagnostic event names and booleans only: never include document content,
// image data, selected filesystem paths or a user's document names.
func (a *App) traceLifecycle(event string, fields map[string]any) {
	a.traceMu.Lock()
	defer a.traceMu.Unlock()
	directory := filepath.Join(a.dataDir, "diagnostics")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return
	}
	path := filepath.Join(directory, "lifecycle-"+a.instance+".jsonl")
	if info, err := os.Stat(path); err == nil && info.Size() >= 1024*1024 {
		return
	}
	entry := map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "event": event}
	for key, value := range fields {
		entry[key] = value
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func (a *App) TraceLifecycle(event string) error {
	if event != "frontend-close-request" {
		return fmt.Errorf("未知诊断事件")
	}
	a.traceLifecycle(event, nil)
	return nil
}
