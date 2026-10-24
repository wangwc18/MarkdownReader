package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// Deny writers and renames until the snapshot check and handle disposition finish.
func openImageSnapshotForRemoval(path string, expected []byte) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.DELETE, windows.FILE_SHARE_READ,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, fmt.Errorf("无法锁定图片进行移动：%w", err)
	}
	file := os.NewFile(uintptr(handle), path)
	failed := true
	defer func() {
		if failed {
			file.Close()
		}
	}()
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		return nil, fmt.Errorf("图片源不能是目录或重解析点")
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 20*1024*1024 {
		return nil, fmt.Errorf("图片必须是20MB以内的普通文件")
	}
	data, err := io.ReadAll(io.LimitReader(file, 20*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, expected) {
		return nil, fmt.Errorf("图片源已被外部修改，取消移动")
	}
	failed = false
	return file, nil
}

func removeImageSnapshot(path string, expected []byte) error {
	file, err := openImageSnapshotForRemoval(path, expected)
	if err != nil {
		return err
	}
	defer file.Close()
	return markImageSnapshotForRemoval(file)
}

func markImageSnapshotForRemoval(file *os.File) error {
	disposition := byte(1) // FILE_DISPOSITION_INFO contains one BOOLEAN.
	return windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfo, &disposition, 1)
}
