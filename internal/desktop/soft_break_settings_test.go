package desktop

import (
	"encoding/json"
	"testing"
)

func TestSoftBreakSettingsDefaultAndLegacyConfiguration(t *testing.T) {
	settings := NewApp(t.TempDir()).settings
	if !settings.Markdown.KeepSingleLineBreaks {
		t.Fatal("single line breaks should remain visible by default")
	}
	if !settings.Markdown.ShowBRMarkers {
		t.Fatal("HTML break markers should be visible by default")
	}
	if err := json.Unmarshal([]byte(`{"markdown":{"mark":false}}`), &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.Markdown.KeepSingleLineBreaks {
		t.Fatal("legacy configuration without this setting changed the default")
	}
	if !settings.Markdown.ShowBRMarkers {
		t.Fatal("legacy configuration changed HTML marker visibility")
	}
	if err := json.Unmarshal([]byte(`{"markdown":{"keepSingleLineBreaks":false}}`), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Markdown.KeepSingleLineBreaks {
		t.Fatal("explicitly disabling single line breaks was ignored")
	}
	if err := json.Unmarshal([]byte(`{"markdown":{"showBRMarkers":false}}`), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Markdown.ShowBRMarkers {
		t.Fatal("explicitly hiding HTML break markers was ignored")
	}
}
