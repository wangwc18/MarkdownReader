package main

import "fmt"

func (a *App) SetDocumentSpelling(id, language string) (string, error) {
	if language != "" && language != "none" && language != "en-US" && language != "en-GB" {
		return "", fmt.Errorf("无效的文档拼写语言")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.docs[id]
	if d == nil {
		return "", fmt.Errorf("文档已关闭")
	}
	d.SpellLanguage = language
	return language, nil
}
