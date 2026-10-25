package desktop

import (
	"path/filepath"
	"strings"
)

// The first pattern includes extensionless files, as observed in Typora 1.3.8.
const markdownFilePattern = "*.;*.mdtxt;*.mdtext;*.apib;*.rmarkdown;*.rmd;*.txt;*.text;*.md;*.markdown;*.mmd;*.mkd;*.mdwn;*.mdown;*.mdx"

func isMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, pattern := range strings.Split(markdownFilePattern, ";") {
		if ext == strings.TrimPrefix(pattern, "*") || (ext == "" && pattern == "*.") {
			return true
		}
	}
	return false
}
