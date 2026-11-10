package config

import (
	"strings"
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

// An operator overrides a built-in string under its key in site.strings, so a
// renamed key would drop that override without a word. These three name the
// welcome note's folded row and the screenshot strip.
func TestTheFoldAndScreenshotKeysKeepTheirNames(t *testing.T) {
	for _, c := range []struct{ key, en, fr string }{
		{"about.summary", "About this site", "À propos de ce site"},
		{"shot.scroll", "Screenshot, scroll sideways", "Capture d'écran, faire défiler"},
		{"shot.full", "See the full screenshot", "Voir la capture entière"},
	} {
		if got := builtinStrings["en"][c.key]; got != c.en {
			t.Errorf("en %q = %q, want %q", c.key, got, c.en)
		}
		if got := builtinStrings["fr"][c.key]; got != c.fr {
			t.Errorf("fr %q = %q, want %q", c.key, got, c.fr)
		}
	}
}

// French sets a no-break space before ? ! : ; so the mark never starts a line
// of its own when the sentence wraps.
func TestFrenchPunctuationKeepsItsNoBreakSpace(t *testing.T) {
	for key, s := range builtinStrings["fr"] {
		for i, r := range s {
			if i > 0 && s[i-1] == ' ' && strings.ContainsRune("?!:;", r) {
				t.Errorf("fr %q has a plain space before %q: %q", key, r, s)
			}
		}
	}
}

// Each language addresses the visitor one way throughout, and the words are the
// ones its own UIs use: the placeholder reads like the search label, and a
// button never says something it does not do.
func TestBuiltinStringsKeepOneRegisterAndTheUsualWords(t *testing.T) {
	for _, c := range []struct{ lang, key, want string }{
		{"es", "leave.title", "Estás saliendo de este directorio"},
		{"es", "leave.body", "Esta dirección pertenece a otra persona. Léela antes de continuar."},
		{"nl", "leave.title", "Je verlaat deze directory"},
		{"nl", "leave.body", "Dit adres is van iemand anders. Lees het voordat je verdergaat."},
		{"nl", "shot.scroll", "Schermafbeelding, scrol opzij"},
		{"nl", "nav.skip", "Naar inhoud springen"},
		{"sv", "about.dismiss", "Dölj"},
		{"sv", "search.placeholder", "Sök efter verktyg…"},
		{"pt", "nav.links", "Ligações"},
		{"de", "nav.theme", "Design"},
		{"it", "nav.skip", "Vai al contenuto"},
		{"it", "host.self", "Self-hosted"},
	} {
		if got := builtinStrings[c.lang][c.key]; got != c.want {
			t.Errorf("%s %q = %q, want %q", c.lang, c.key, got, c.want)
		}
	}
}

var stateNames = []string{"soon", "retired", "beta", "deprecated", "new"}

// The completeness test only compares tables with each other, so a note key
// misspelled the same way in all eight would pass it. This holds the names.
func TestEveryStateHasANoteInEveryLocale(t *testing.T) {
	for lang, table := range builtinStrings {
		for _, s := range stateNames {
			if table["state."+s+".note"] == "" {
				t.Errorf("locale %q has no note for state %s", lang, s)
			}
		}
	}
}

// The French notes carry U+202F before the colon and U+2019 as apostrophe;
// a plain space or a straight quote looks the same in an editor, so both are
// asserted by rune and not only through the string compare.
func TestTheEnglishAndFrenchNotesAreTheApprovedOnes(t *testing.T) {
	for _, c := range []struct{ state, en, fr string }{
		{"soon", "Not open yet: it will be available soon.", "Pas encore ouvert : il sera disponible prochainement."},
		{"retired", "This service has been shut down and is no longer available.", "Ce service a été arrêté et n’est plus accessible."},
		{"beta", "Available, but still being tested: it may change.", "Disponible, mais encore en test : son fonctionnement peut évoluer."},
		{"deprecated", "Still available, but scheduled to be withdrawn.", "Encore accessible, mais appelé à être retiré."},
		{"new", "Recently added to this directory.", "Ajouté récemment à cet annuaire."},
	} {
		key := "state." + c.state + ".note"
		if got := builtinStrings["en"][key]; got != c.en {
			t.Errorf("en %q = %q, want %q", key, got, c.en)
		}
		if got := builtinStrings["fr"][key]; got != c.fr {
			t.Errorf("fr %q = %q, want %q", key, got, c.fr)
		}
	}
	if s := builtinStrings["fr"]["state.soon.note"]; !strings.ContainsRune(s, ' ') {
		t.Errorf("fr state.soon.note has no U+202F before its colon: %q", s)
	}
	if s := builtinStrings["fr"]["state.retired.note"]; !strings.ContainsRune(s, '’') || strings.ContainsRune(s, '\'') {
		t.Errorf("fr state.retired.note does not use U+2019 as its apostrophe: %q", s)
	}
}

// Same rule as the state notes: U+202F and U+2019 look like a space and a
// straight quote in an editor, so they are asserted by rune.
func TestTheOnDemandStringsAreTheApprovedOnes(t *testing.T) {
	for _, c := range []struct{ key, en, fr string }{
		{"ondemand.label", "On demand", "À la demande"},
		{"ondemand.note", "Starts when opened: the first load may take a few seconds.", "Démarre à l’ouverture : le premier chargement peut prendre quelques secondes."},
	} {
		if got := builtinStrings["en"][c.key]; got != c.en {
			t.Errorf("en %q = %q, want %q", c.key, got, c.en)
		}
		if got := builtinStrings["fr"][c.key]; got != c.fr {
			t.Errorf("fr %q = %q, want %q", c.key, got, c.fr)
		}
	}
	s := builtinStrings["fr"]["ondemand.note"]
	if !strings.ContainsRune(s, ' ') {
		t.Errorf("fr ondemand.note has no U+202F before its colon: %q", s)
	}
	if !strings.ContainsRune(s, '’') || strings.ContainsRune(s, '\'') {
		t.Errorf("fr ondemand.note does not use U+2019 as its apostrophe: %q", s)
	}
}
