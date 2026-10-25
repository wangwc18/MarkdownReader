package desktop

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Create exclusively: neither a previous insertion nor a concurrent window
// may cause a different existing image to be overwritten.
func copyInsertedImage(source, folder string) (string, error) {
	data, err := readCopyImage(source)
	if err != nil {
		return "", err
	}
	name := filepath.Base(source)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext) + "-" + digest(data)[:8]
	for serial := 0; serial < 2000; serial++ {
		candidate := name
		if serial == 1 {
			candidate = stem + ext
		}
		if serial > 1 {
			candidate = fmt.Sprintf("%s-%d%s", stem, serial-1, ext)
		}
		target := filepath.Join(folder, candidate)
		info, statErr := os.Lstat(target)
		if statErr == nil {
			if info.Mode().IsRegular() {
				existing, readErr := readCopyImage(target)
				if readErr != nil {
					return "", readErr
				}
				if bytes.Equal(existing, data) {
					return target, nil
				}
			}
			continue
		}
		if !os.IsNotExist(statErr) {
			return "", statErr
		}
		if err = createRestoredFile(target, data, 0644); err == nil {
			return target, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
		// Another window claimed this name. Retry the same slot to check whether
		// it stored identical bytes before allocating a further name.
		serial--
	}
	return "", fmt.Errorf("同名图片过多，请更换图片目录")
}
