package desktop

import (
	"encoding/json"
	"testing"
)

func TestInlineMathDigitSettingsCompatibilityAndPersistence(t *testing.T) {
	root := t.TempDir()
	a := NewApp(root)
	if err := a.initStorage(); err != nil {
		t.Fatal(err)
	}
	s := a.GetSettings()
	if s.Markdown.InlineMathDigit {
		t.Fatal("numeric inline math should remain disabled by default")
	}
	if err := json.Unmarshal([]byte(`{"markdown":{"mark":true}}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Markdown.InlineMathDigit {
		t.Fatal("legacy configuration changed the default")
	}
	for _, enabled := range []bool{true, false} {
		s.Markdown.InlineMathDigit = enabled
		if err := a.SetSettings(s); err != nil {
			t.Fatal(err)
		}
		reopened := NewApp(root)
		if err := reopened.initStorage(); err != nil {
			t.Fatal(err)
		}
		if reopened.GetSettings().Markdown.InlineMathDigit != enabled {
			t.Fatalf("numeric inline math setting did not persist: %v", enabled)
		}
	}
}
