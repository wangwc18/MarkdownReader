package main

import (
	"embed"
	"markdownreader/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	desktop.Run(assets)
}
