package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonalDictionaryPersistenceAndWindowMerge(t *testing.T) {
	a := fixture(t)
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddPersonalWord("quikc"); err != nil {
		t.Fatal(err)
	}
	// b still has its original empty map: its addition must retain a's word.
	result, err := b.AddPersonalWord("mispeling")
	if err != nil {
		t.Fatal(err)
	}
	if !result.PersonalWords["quikc"] || !result.PersonalWords["mispeling"] {
		t.Fatalf("lost concurrent addition: %v", result.PersonalWords)
	}
	if _, err = a.AddPersonalWord("quikc"); err != nil {
		t.Fatal(err)
	}
	c := NewApp(a.base)
	if err = c.initStorage(); err != nil {
		t.Fatal(err)
	}
	words := c.GetSettings().PersonalWords
	if len(words) != 2 || !words["quikc"] || !words["mispeling"] {
		t.Fatalf("reload lost words: %v", words)
	}
	words["quikc"] = false
	if !c.GetSettings().PersonalWords["quikc"] {
		t.Fatal("snapshot exposes mutable dictionary")
	}
}

func TestPersonalDictionaryValidationAndFailedWrite(t *testing.T) {
	a := fixture(t)
	for _, word := range []string{"", "two words", "<script>", "../word", strings.Repeat("a", 65), "中文"} {
		if _, err := a.AddPersonalWord(word); err == nil {
			t.Fatalf("accepted %q", word)
		}
	}
	s, err := a.AddPersonalWord("it’s")
	if err != nil || !s.PersonalWords["it's"] {
		t.Fatalf("apostrophe normalization: %v %v", s.PersonalWords, err)
	}
	path := filepath.Join(a.dataDir, "settings.json")
	if err = os.WriteFile(path, []byte("broken-settings"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.AddPersonalWord("quikc"); err == nil {
		t.Fatal("accepted failed persistence")
	}
	if a.GetSettings().PersonalWords["quikc"] {
		t.Fatal("failed write changed active dictionary")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "broken-settings" {
		t.Fatal("corrupt settings overwritten")
	}
}
