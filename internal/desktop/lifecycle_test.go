package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleDiagnosticsContainOnlyExplicitEvents(t *testing.T) {
	a := NewApp(t.TempDir())
	a.traceLifecycle("started", nil)
	a.traceLifecycle("copy-all-dialog-return", map[string]any{"selected": false, "error": false})
	if err := a.TraceLifecycle("frontend-close-request"); err != nil {
		t.Fatal(err)
	}
	if err := a.TraceLifecycle("arbitrary document content"); err == nil {
		t.Fatal("unbounded frontend trace accepted")
	}
	data, err := os.ReadFile(filepath.Join(a.dataDir, "diagnostics", "lifecycle-"+a.instance+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("unexpected events: %s", data)
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["time"] == nil || entry["event"] == nil {
			t.Fatal("missing diagnostic fields")
		}
	}
}
