package desktop

import (
	"bytes"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestSingleImageMoveAndOverwriteHistory(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new target", true: "overwrite"}[existing], func(t *testing.T) {
			a := fixture(t)
			source := filepath.Join(a.base, "source.png")
			target := filepath.Join(a.base, "target.png")
			data := clipboardPNG(t)
			old := []byte("old target")
			if err := os.WriteFile(source, data, 0644); err != nil {
				t.Fatal(err)
			}
			if existing {
				if err := os.WriteFile(target, old, 0644); err != nil {
					t.Fatal(err)
				}
			}
			docPath := filepath.Join(a.base, "doc.md")
			text := "![first](source.png)\n![second](source.png)\n"
			if err := os.WriteFile(docPath, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			d, err := a.OpenPath(docPath)
			if err != nil {
				t.Fatal(err)
			}
			result, err := a.moveImageTo(d.ID, "source.png", target, true)
			if err != nil {
				t.Fatal(err)
			}
			if result.Token == "" || result.Original != text || result.Reference == "" {
				t.Fatalf("bad result: %+v", result)
			}
			if a.docs[d.ID].Content != text {
				t.Fatal("backend mutated Markdown")
			}
			if _, err := os.Stat(source); !os.IsNotExist(err) {
				t.Fatalf("source remains: %v", err)
			}
			if got, _ := os.ReadFile(target); !bytes.Equal(got, data) {
				t.Fatal("target mismatch")
			}
			if err := a.SetImageCopyState(d.ID, result.Token, false); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(source); !bytes.Equal(got, data) {
				t.Fatal("source not restored")
			}
			if existing {
				if got, _ := os.ReadFile(target); !bytes.Equal(got, old) {
					t.Fatal("overwritten target not restored")
				}
			} else if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("new target remains after undo")
			}
			if err := a.SetImageCopyState(d.ID, result.Token, true); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(source); !os.IsNotExist(err) {
				t.Fatal("redo did not move source")
			}
		})
	}
}

func TestMoveReadOnlySourceRollsBackDestination(t *testing.T) {
	a := fixture(t)
	source := filepath.Join(a.base, "readonly.png")
	target := filepath.Join(a.base, "target.png")
	data := clipboardPNG(t)
	old := []byte("previous destination")
	if err := os.WriteFile(source, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, old, 0644); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	defer windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_NORMAL)
	path := filepath.Join(a.base, "doc.md")
	if err := os.WriteFile(path, []byte("![a](readonly.png)"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.moveImageTo(d.ID, "readonly.png", target, true); err == nil {
		t.Fatal("readonly source unexpectedly moved")
	}
	if got, _ := os.ReadFile(source); !bytes.Equal(got, data) {
		t.Fatal("failed move changed source")
	}
	if got, _ := os.ReadFile(target); !bytes.Equal(got, old) {
		t.Fatal("failed move did not restore previous destination")
	}
	if len(a.imageCopies) != 0 {
		t.Fatal("failed move retained incomplete history")
	}
}

func TestMoveIdenticalTargetAndSameFile(t *testing.T) {
	a := fixture(t)
	data := clipboardPNG(t)
	source := filepath.Join(a.base, "source.png")
	target := filepath.Join(a.base, "target.png")
	for _, path := range []string{source, target} {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	docPath := filepath.Join(a.base, "doc.md")
	if err := os.WriteFile(docPath, []byte("![a](source.png)"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := a.OpenPath(docPath)
	if err != nil {
		t.Fatal(err)
	}
	same, err := a.moveImageTo(d.ID, "source.png", source, false)
	if err != nil || same.Token != "" {
		t.Fatalf("same file: %+v %v", same, err)
	}
	result, err := a.moveImageTo(d.ID, "source.png", target, false)
	if err != nil || result.Token == "" {
		t.Fatalf("identical target: %+v %v", result, err)
	}
	if err := a.SetImageCopyState(d.ID, result.Token, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{source, target} {
		if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
			t.Fatal("undo changed preexisting identical file")
		}
	}
	prepared, err := a.localImageMoveSource(d.ID, "source.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("changed after dialog snapshot"), 0644); err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(a.base, "never-created.png")
	if _, err := a.copyImageSourceTo(d.ID, prepared, absent, false, true); err == nil {
		t.Fatal("changed prepared source accepted")
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("changed source caused target write")
	}
}
