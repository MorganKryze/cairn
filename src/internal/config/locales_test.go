package config

import (
	"testing"
)

// A locale that reaches a page is written into markup as a lang attribute, so
// what counts as one is a safety question as much as a spelling one: the
// pattern admits letters, digits and dashes, and therefore no quote, space or
// angle bracket that could end the attribute early.
func TestIsLocaleAndSameLanguage(t *testing.T) {
	for _, c := range []struct {
		s    string
		want bool
	}{
		{"fr", true}, {"pt-BR", true}, {"AR", true},
		{"", false}, {"1fr", false}, {`fr" onload="x`, false}, {"fr fr", false}, {"fr>", false},
	} {
		if got := IsLocale(c.s); got != c.want {
			t.Errorf("IsLocale(%q) = %v, want %v", c.s, got, c.want)
		}
	}
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"pt", "pt-BR", true}, {"pt-BR", "PT", true}, {"fr", "fr", true},
		{"fr", "en", false}, {"ar", "fa", false}, {"", "fr", false},
	} {
		if got := SameLanguage(c.a, c.b); got != c.want {
			t.Errorf("SameLanguage(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestLocaleTablesComplete(t *testing.T) {
	want := builtinStrings["en"]
	for lang, table := range builtinStrings {
		if len(table) != len(want) {
			t.Errorf("locale %q has %d strings, en has %d", lang, len(table), len(want))
		}
		for key := range want {
			if table[key] == "" {
				t.Errorf("locale %q is missing %q", lang, key)
			}
		}
	}
}
