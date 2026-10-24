// Offline integration fixture: returns local file URIs without uploading data.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--fail" {
		fmt.Fprintln(os.Stderr, "离线上传测试失败")
		os.Exit(7)
	}
	fmt.Println("Offline Upload Fixture:")
	args := os.Args[1:]
	remote := len(args) > 0 && args[0] == "--remote"
	if remote {
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "upload" {
		args = args[1:]
	}
	store := ""
	if len(args) >= 2 && args[0] == "--store" {
		store, args = args[1], args[2:]
		if err := os.MkdirAll(store, 0755); err != nil {
			panic(err)
		}
	}
	for i, path := range args {
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(8)
		}
		if remote {
			fmt.Printf("https://example.invalid/offline-%d.png\n", i)
		} else {
			if store != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					panic(err)
				}
				target := filepath.Join(store, fmt.Sprintf("fixture-%x%s", sha256.Sum256(data), filepath.Ext(path)))
				file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
				if os.IsExist(err) {
					existing, readErr := os.ReadFile(target)
					if readErr != nil || !bytes.Equal(existing, data) {
						panic("fixture destination differs")
					}
				} else if err != nil {
					panic(err)
				} else {
					_, writeErr := file.Write(data)
					closeErr := file.Close()
					if writeErr != nil {
						panic(writeErr)
					}
					if closeErr != nil {
						panic(closeErr)
					}
				}
				path = target
			}
			fmt.Println((&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}).String())
		}
	}
}
