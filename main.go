package main

import (
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	win "golang.org/x/sys/windows"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	base := filepath.Dir(exe)
	app := NewApp(base)
	if err = app.initStorage(); err != nil {
		fatal("程序目录不可写，请解压到可写目录后重试。\n" + err.Error())
		return
	}
	app.traceLifecycle("started", nil)
	defer app.traceLifecycle("main-return", nil)
	cleanupCrash, crashErr := configureCrashOutput(app.dataDir, app.instance)
	app.traceLifecycle("crash-output", map[string]any{"enabled": crashErr == nil})
	if crashErr == nil {
		defer cleanupCrash()
	}
	browser := filepath.Join(base, "runtime")
	if _, err = os.Stat(filepath.Join(browser, "msedgewebview2.exe")); err != nil {
		browser = ""
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(err.Error())
		return
	}
	app.debugPort = listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	os.Setenv("MARKDOWNREADER_BROWSER_ARGUMENTS", fmt.Sprintf("--remote-debugging-port=%d --remote-debugging-address=127.0.0.1", app.debugPort))
	err = wails.Run(&options.App{Title: "MarkdownReader", Width: 1100, Height: 800, MinWidth: 640, MinHeight: 480,
		AssetServer: &assetserver.Options{Assets: assets, Handler: app},
		OnStartup:   app.startup, OnBeforeClose: app.beforeClose, Bind: []interface{}{app},
		Windows: &windows.Options{WebviewBrowserPath: browser, WebviewUserDataPath: filepath.Join(app.dataDir, "webview", app.instance), Theme: windows.Light, WindowClassName: "MarkdownReader"},
	})
	app.traceLifecycle("runtime-return", map[string]any{"error": err != nil})
	if err != nil {
		fatal(err.Error())
	}
	profile := filepath.Join(app.dataDir, "webview", app.instance)
	if rel, e := filepath.Rel(filepath.Join(app.dataDir, "webview"), profile); e == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		for i := 0; i < 5; i++ {
			if os.RemoveAll(profile) == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func fatal(message string) {
	text, _ := win.UTF16PtrFromString(message)
	title, _ := win.UTF16PtrFromString("MarkdownReader")
	win.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
