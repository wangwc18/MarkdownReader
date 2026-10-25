package desktop

import (
	"fmt"
	"regexp"
	"strings"
)

var personalWordPattern = regexp.MustCompile(`^[A-Za-z]+('[A-Za-z]+)*$`)

func canonicalPersonalWord(word string) (string, error) {
	word = strings.ReplaceAll(word, "’", "'")
	if len(word) > 64 || !personalWordPattern.MatchString(word) {
		return "", fmt.Errorf("个人词典目前支持不超过64字符的英文单词")
	}
	return word, nil
}

func validatePersonalWords(words map[string]bool) error {
	if len(words) > 10000 {
		return fmt.Errorf("个人词典最多支持10000个词")
	}
	for word := range words {
		canonical, err := canonicalPersonalWord(word)
		if err != nil {
			return err
		}
		if canonical != word {
			return fmt.Errorf("个人词典单词格式无效")
		}
	}
	return nil
}

// Settings persistence merges maps per key while holding the shared disk lock.
// Independent additions in separate windows therefore survive stale snapshots.
func (a *App) AddPersonalWord(word string) (Settings, error) {
	canonical, err := canonicalPersonalWord(word)
	if err != nil {
		return Settings{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	previous := a.settingsSnapshot()
	a.settings.PersonalWords = a.settingsSnapshot().PersonalWords
	a.settings.PersonalWords[canonical] = true
	if err = validatePersonalWords(a.settings.PersonalWords); err == nil {
		err = a.persistSettings()
	}
	if err != nil {
		a.settings = previous
		return Settings{}, err
	}
	return a.settingsSnapshot(), nil
}
