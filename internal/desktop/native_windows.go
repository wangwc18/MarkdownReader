package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	win "golang.org/x/sys/windows"
)

func hideConversionConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: win.CREATE_NO_WINDOW}
}

type shellFileOperation struct {
	Window    uintptr
	Operation uint32
	From      *uint16
	To        *uint16
	Flags     uint16
	Aborted   int32
	Mappings  uintptr
	Title     *uint16
}

func recycleFile(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "*?\x00") || strings.HasPrefix(path, `\\?\`) {
		return errors.New("回收站需要不含通配符的完整普通路径")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("仅支持回收普通文件")
	}
	from, err := win.UTF16FromString(path)
	if err != nil {
		return err
	}
	from = append(from, 0)
	op := shellFileOperation{Operation: 3, From: &from[0], Flags: 0x0040 | 0x0004 | 0x0010 | 0x0400}
	result, _, _ := win.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW").Call(uintptr(unsafe.Pointer(&op)))
	runtime.KeepAlive(from)
	if result != 0 || op.Aborted != 0 {
		return fmt.Errorf("Windows 回收操作未完成（代码 %d，取消 %d）", result, op.Aborted)
	}
	return nil
}

var nativeUser32 = win.NewLazySystemDLL("user32.dll")
var nativeEnumMutex sync.Mutex
var nativeEnumHandles []uintptr
var nativeEnumCallback = syscall.NewCallback(func(handle uintptr, _ uintptr) uintptr {
	nativeEnumHandles = append(nativeEnumHandles, handle)
	return 1
})

type NativeWindow struct {
	Handle  uintptr `json:"handle"`
	Title   string  `json:"title"`
	Current bool    `json:"current"`
}

func (a *App) Windows() ([]NativeWindow, error) {
	nativeEnumMutex.Lock()
	nativeEnumHandles = nil
	ok, _, err := nativeUser32.NewProc("EnumWindows").Call(nativeEnumCallback, 0)
	handles := append([]uintptr(nil), nativeEnumHandles...)
	nativeEnumHandles = nil
	nativeEnumMutex.Unlock()
	if ok == 0 {
		return nil, err
	}
	current, _, _ := nativeUser32.NewProc("GetForegroundWindow").Call()
	list := []NativeWindow{}
	for _, handle := range handles {
		visible, _, _ := nativeUser32.NewProc("IsWindowVisible").Call(handle)
		if visible == 0 {
			continue
		}
		class := make([]uint16, 128)
		nativeUser32.NewProc("GetClassNameW").Call(handle, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if win.UTF16ToString(class) != "MarkdownReader" {
			continue
		}
		var pid uint32
		nativeUser32.NewProc("GetWindowThreadProcessId").Call(handle, uintptr(unsafe.Pointer(&pid)))
		process, err := win.OpenProcess(win.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			continue
		}
		name := make([]uint16, 32768)
		size := uint32(len(name))
		queryError := win.QueryFullProcessImageName(process, 0, &name[0], &size)
		win.CloseHandle(process)
		if queryError != nil {
			continue
		}
		if !strings.EqualFold(filepath.Base(win.UTF16ToString(name[:size])), "MarkdownReader.exe") {
			continue
		}
		title := make([]uint16, 2048)
		nativeUser32.NewProc("GetWindowTextW").Call(handle, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		list = append(list, NativeWindow{Handle: handle, Title: win.UTF16ToString(title), Current: handle == current})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Handle < list[j].Handle })
	return list, nil
}
func (a *App) ActivateWindow(handle uintptr) error {
	windows, err := a.Windows()
	if err != nil {
		return err
	}
	valid := false
	for _, window := range windows {
		if window.Handle == handle {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("目标不是当前 MarkdownReader 窗口")
	}
	iconic, _, _ := nativeUser32.NewProc("IsIconic").Call(handle)
	if iconic != 0 {
		nativeUser32.NewProc("ShowWindow").Call(handle, 9)
	}
	ok, _, _ := nativeUser32.NewProc("SetForegroundWindow").Call(handle)
	if ok == 0 {
		return errors.New("Windows 未允许切换前台窗口，请从任务栏选择")
	}
	return nil
}
func (a *App) CycleWindow(backward bool) error {
	windows, err := a.Windows()
	if err != nil || len(windows) < 2 {
		return err
	}
	index := 0
	for i, window := range windows {
		if window.Current {
			index = i
			break
		}
	}
	direction := 1
	if backward {
		direction = -1
	}
	return a.ActivateWindow(windows[(index+direction+len(windows))%len(windows)].Handle)
}
