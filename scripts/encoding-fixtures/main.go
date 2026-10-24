package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

func main() {
	root := filepath.Join("examples", "编码示例")
	if err := os.MkdirAll(root, 0755); err != nil {
		panic(err)
	}
	gbk, err := simplifiedchinese.GBK.NewEncoder().String("# 中文 GBK\r\n\r\n中文编码保存往返。\r\n")
	if err != nil {
		panic(err)
	}
	big5, err := traditionalchinese.Big5.NewEncoder().String("# 繁體中文\r\n\r\n編碼測試。\r\n")
	if err != nil {
		panic(err)
	}
	western, err := charmap.Windows1252.NewEncoder().String("# Café\r\n\r\nRésumé €\r\n")
	if err != nil {
		panic(err)
	}
	utf16bytes := []byte{}
	for _, word := range utf16.Encode([]rune("# 中文 UTF16\r\n\r\n无 BOM 编码。\r\n")) {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], word)
		utf16bytes = append(utf16bytes, b[:]...)
	}
	files := map[string][]byte{"GBK 中文.md": []byte(gbk), "Big5 繁體.md": []byte(big5), "Windows-1252.md": []byte(western), "UTF16 无BOM.md": utf16bytes, "UTF8 编码.md": []byte("# 编码检查\n\n中文 😀\n")}
	for name, b := range files {
		path := filepath.Join(root, name)
		if len(os.Args) > 1 && os.Args[1] == "--check" {
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, b) {
				panic("编码样本未恢复原始字节：" + path)
			}
			fmt.Println("verified", path)
			continue
		}
		if err = os.WriteFile(path, b, 0644); err != nil {
			panic(err)
		}
		fmt.Println(path)
	}
}
