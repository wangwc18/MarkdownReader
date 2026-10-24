package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
)

// The runtime owns a duplicate descriptor, so a GUI launch without stderr can
// still retain an unhandled Go panic. This does not intercept native exits.
func configureCrashOutput(dataDir, instance string) (func(), error) {
	directory := filepath.Join(dataDir, "diagnostics")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "crash-"+instance+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	err = debug.SetCrashOutput(file, debug.CrashOptions{})
	file.Close()
	if err != nil {
		os.Remove(path)
		return nil, err
	}
	return func() {
		debug.SetCrashOutput(nil, debug.CrashOptions{})
		if info, err := os.Stat(path); err == nil && info.Size() == 0 {
			os.Remove(path)
		}
	}, nil
}
