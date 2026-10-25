package desktop

import "testing"

func TestSpellingLanguageDocumentIsolation(t *testing.T) {
	a := fixture(t)
	one, two := a.NewDocument(), a.NewDocument()
	original := *one
	if _, err := a.SetDocumentSpelling(one.ID, "en-GB"); err != nil {
		t.Fatal(err)
	}
	if a.docs[one.ID].SpellLanguage != "en-GB" || a.docs[two.ID].SpellLanguage != "" {
		t.Fatal("document choices leak")
	}
	if one.Content != original.Content || one.Dirty != original.Dirty || one.Revision != original.Revision {
		t.Fatal("language changed content/history")
	}
	if _, err := a.SetDocumentSpelling(one.ID, "none"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetDocumentSpelling(one.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetDocumentSpelling(one.ID, "invalid"); err == nil {
		t.Fatal("accepted unknown language")
	}
	if _, err := a.SetDocumentSpelling("closed", "en-US"); err == nil {
		t.Fatal("accepted closed document")
	}
}

func TestSpellingDefaultLanguagePersists(t *testing.T) {
	a := fixture(t)
	s := a.GetSettings()
	s.SpellLanguage = "en-GB"
	if err := a.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	b := NewApp(a.base)
	if err := b.initStorage(); err != nil {
		t.Fatal(err)
	}
	if b.GetSettings().SpellLanguage != "en-GB" {
		t.Fatal("default language not persisted")
	}
	s.SpellLanguage = "unsupported"
	if err := a.SetSettings(s); err == nil {
		t.Fatal("accepted unsupported default")
	}
}
