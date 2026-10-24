// Local desktop regression converter: GBK text on stdout, or stderr + failure.
package main

import (
	"io"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) > 2 && (os.Args[1] == "--folder" || os.Args[1] == "--folder-fail") {
		output := os.Args[2]
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(9)
		}
		if err := os.MkdirAll(filepath.Join(output, "资源"), 0700); err != nil {
			os.Exit(10)
		}
		if err := os.WriteFile(filepath.Join(output, "报告.md"), data, 0600); err != nil {
			os.Exit(11)
		}
		if err := os.WriteFile(filepath.Join(output, "资源", "说明.txt"), []byte("离线资源"), 0600); err != nil {
			os.Exit(12)
		}
		os.Stdout.WriteString("文件夹输出完成")
		if os.Args[1] == "--folder-fail" {
			os.Exit(7)
		}
		return
	}
	data := []byte{0xd6, 0xd0, 0xce, 0xc4, 0xcd, 0xea, 0xb3, 0xc9} // 中文完成, GBK
	if len(os.Args) > 1 && os.Args[1] == "--fail" {
		os.Stderr.Write(data)
		os.Exit(7)
	}
	os.Stdout.Write(data)
}
