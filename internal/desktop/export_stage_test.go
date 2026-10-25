package desktop

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// Create a directory junction without a shell or symlink privileges.
// Layout: Microsoft REPARSE_DATA_BUFFER MountPointReparseBuffer.
func testDirectoryJunction(link, target string) error {
	if err := os.Mkdir(link, 0700); err != nil {
		return err
	}
	path, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	sub := utf16.Encode([]rune(`\??\` + target))
	print := utf16.Encode([]rune(target))
	payload := append(append(append(sub, 0), print...), 0)
	data := make([]byte, 16+len(payload)*2)
	binary.LittleEndian.PutUint32(data, 0xa0000003)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8))
	binary.LittleEndian.PutUint16(data[10:], uint16(len(sub)*2))
	binary.LittleEndian.PutUint16(data[12:], uint16((len(sub)+1)*2))
	binary.LittleEndian.PutUint16(data[14:], uint16(len(print)*2))
	for i, c := range payload {
		binary.LittleEndian.PutUint16(data[16+i*2:], c)
	}
	var returned uint32
	return windows.DeviceIoControl(handle, 0x000900a4, &data[0], uint32(len(data)), nil, 0, &returned, nil)
}

func TestExportStageRegularTreeCleanup(t *testing.T) {
	workspace, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	stage, err := newExportStage(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if err := stage.root.Mkdir("子目录", 0700); err != nil {
		t.Fatal(err)
	}
	if err := stage.root.WriteFile("子目录/输出.txt", []byte("中文"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := stage.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := stage.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(stage.Path); !os.IsNotExist(err) {
		t.Fatal("regular stage remains", err)
	}
}

func TestExportStageRejectsConverterJunctionAndPreservesTarget(t *testing.T) {
	workspace := t.TempDir()
	sentinel := filepath.Join(workspace, "must-stay.txt")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	stage, err := newExportStage(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	input := filepath.Join(t.TempDir(), "input.md")
	if err := os.WriteFile(input, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestExportProgramFixture$", "markdownreader-export-fixture", "junction", stage.Path, input, workspace)
	hideConversionConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("junction converter: %v %s", err, out)
	}
	if err := stage.Verify(); err == nil {
		t.Fatal("junction output accepted")
	}
	if err := stage.Cleanup(); err == nil {
		t.Fatal("junction stage cleaned instead of preserved")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "unchanged" {
		t.Fatal("junction target changed", err)
	}
	link := filepath.Join(stage.Path, "linked")
	if info, err := os.Lstat(link); err != nil || !reparseFile(info) {
		t.Fatal("junction not preserved", err)
	}
	// Unlink only the known junction entry; no traversal or recursive cleanup.
	if err := stage.root.Remove("linked"); err != nil {
		t.Fatal(err)
	}
	if err := stage.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestExportStageRejectsReplacedRoot(t *testing.T) {
	stage, err := newExportStage()
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "must-stay.txt")
	os.WriteFile(sentinel, []byte("unchanged"), 0600)
	moved := stage.Path + "-moved"
	if filepath.Dir(moved) != stage.parentPath || stage.verifyIdentity() != nil {
		t.Fatal("unverified rename target")
	}
	if err := stage.parent.Rename(stage.leaf, filepath.Base(moved)); err != nil {
		t.Fatal(err)
	}
	if err := testDirectoryJunction(stage.Path, outside); err != nil {
		t.Fatal(err)
	}
	if err := stage.Cleanup(); err == nil || !strings.Contains(err.Error(), "根目录") {
		t.Fatal("replaced root not rejected", err)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "unchanged" {
		t.Fatal("outside sentinel changed", err)
	}
	if err := stage.parent.Remove(stage.leaf); err != nil {
		t.Fatal(err)
	}
	if err := stage.parent.Rename(filepath.Base(moved), stage.leaf); err != nil {
		t.Fatal(err)
	}
	if err := stage.Cleanup(); err != nil {
		t.Fatal(err)
	}
}
