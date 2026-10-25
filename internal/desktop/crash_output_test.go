package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrashOutputChild(t *testing.T) {
	mode := os.Getenv("MARKDOWNREADER_CRASH_TEST_MODE")
	if mode == "" {
		return
	}
	cleanup, err := configureCrashOutput(os.Getenv("MARKDOWNREADER_CRASH_TEST_DIR"), "fixture")
	if err != nil {
		os.Exit(9)
	}
	if mode == "panic" {
		panic("markdownreader-owned-crash-fixture")
	}
	cleanup()
	os.Exit(0)
}

func TestGUICrashOutputCapturesRuntimePanic(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"panic", "normal"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			command := exec.Command(exe, "-test.run=^TestCrashOutputChild$")
			command.Env = append(os.Environ(), "MARKDOWNREADER_CRASH_TEST_MODE="+mode, "MARKDOWNREADER_CRASH_TEST_DIR="+directory)
			hideConversionConsole(command)
			err := command.Run()
			path := filepath.Join(directory, "diagnostics", "crash-fixture.log")
			if mode == "panic" {
				if err == nil {
					t.Fatal("panic child succeeded")
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil || !strings.Contains(string(data), "markdownreader-owned-crash-fixture") || !strings.Contains(string(data), "TestCrashOutputChild") {
					t.Fatalf("crash output missing: %s %v", data, readErr)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err = os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("empty crash file retained")
				}
			}
		})
	}
}

func TestCrashOutputExistingFileIsPreserved(t *testing.T) {
	directory := t.TempDir()
	folder := filepath.Join(directory, "diagnostics")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "crash-fixture.log")
	if err := os.WriteFile(path, []byte("previous report"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := configureCrashOutput(directory, "fixture"); err == nil {
		t.Fatal("existing report overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "previous report" {
		t.Fatal("previous report changed", err)
	}
}
