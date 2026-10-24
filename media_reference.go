package main

import (
	"net/url"
	"path/filepath"
)

// A reference is a URI, not a filesystem path: # and % in names must remain
// part of the filename instead of becoming fragments or escape sequences.
func imageReference(root, path string, relative bool) (string, error) {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	path = canonical
	if !relative {
		return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}).String(), nil
	}
	// Windows may present a directory using its short 8.3 alias or long name.
	// Compare canonical paths, otherwise a child image becomes a long ../ path.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return (&url.URL{Path: filepath.ToSlash(rel)}).EscapedPath(), nil
}
