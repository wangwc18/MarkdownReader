//go:build windows

package edge

import "testing"

func TestMarkdownReaderInspectorOrigin(t *testing.T) {
	for _, source := range []string{"http://wails.localhost/", "http://wails.localhost/?document=1#view"} {
		if !markdownReaderOrigin(source) {
			t.Fatalf("own origin rejected: %s", source)
		}
	}
	for _, source := range []string{"https://wails.localhost/", "http://wails.localhost.example/", "http://user@wails.localhost/", "http://wails.localhost:80/", "file:///test.html", "http://example.com/", "%", "//wails.localhost/"} {
		if markdownReaderOrigin(source) {
			t.Fatalf("foreign origin accepted: %s", source)
		}
	}
}
