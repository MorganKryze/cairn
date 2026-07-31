package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A translated field falls back through the base language, then the site
// default, then any value at all: a half-translated config still renders.
func TestLStringFallback(t *testing.T) {
	for _, c := range []struct {
		name        string
		ls          LString
		locale, def string
		want        string
	}{
		{"exact match wins", LString{"fr": "Salut", "en": "Hi"}, "fr", "en", "Salut"},
		{"regional finds its base", LString{"pt": "Olá"}, "pt-BR", "en", "Olá"},
		{"unknown locale takes the default", LString{"en": "Hi"}, "de", "en", "Hi"},
		{"a bare string serves every locale", LString{"": "Pad"}, "fr", "en", "Pad"},
		{"nothing at all is empty", LString{}, "fr", "en", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.ls.Get(c.locale, c.def); got != c.want {
				t.Errorf("Get(%q, %q) = %q, want %q", c.locale, c.def, got, c.want)
			}
		})
	}
}

// Get alone cannot tell a translation from a fallback, and a page that cannot
// tell announces another language's wording as its own: the wrong voice on a
// screen reader, and the wrong way round for a right-to-left language. Get is
// written in terms of GetLocale, so the two can never answer from different
// lookup orders.
func TestGetLocaleReportsWhereTheWordingCameFrom(t *testing.T) {
	for _, c := range []struct {
		name               string
		ls                 LString
		locale, def        string
		wantText, wantFrom string
	}{
		{"a translation comes from the locale asked for", LString{"fr": "Salut", "en": "Hi"}, "fr", "en", "Salut", "fr"},
		{"the site default is a fallback and says so", LString{"en": "Hi"}, "de", "en", "Hi", "en"},
		{"the last resort names the language it found", LString{"ar": "مرحبا", "he": "שלום"}, "fr", "fr", "مرحبا", "ar"},
		{"one string for every locale is never a fallback", LString{"": "Pad"}, "fr", "en", "Pad", "fr"},
		{"nothing at all comes from nowhere", LString{}, "fr", "en", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			text, from := c.ls.GetLocale(c.locale, c.def)
			if text != c.wantText || from != c.wantFrom {
				t.Errorf("GetLocale(%q, %q) = %q, %q, want %q, %q", c.locale, c.def, text, from, c.wantText, c.wantFrom)
			}
			if got := c.ls.Get(c.locale, c.def); got != text {
				t.Errorf("Get says %q where GetLocale says %q", got, text)
			}
		})
	}
}

// A services file that is not a list of services has to say so in the terms of
// the file, naming what was found where.
func TestParseServicesRefusals(t *testing.T) {
	for _, c := range []struct {
		name, yaml string
		wants      []string
	}{
		{"a mapping instead of a list", "pad:\n  url: https://pad.example.org\n", []string{"services.yaml", "list"}},
		{"a service without an id", "- {url: https://pad.example.org, name: Pad}\n", []string{"id"}},
		{"a service without a url", "- {id: pad, name: Pad}\n", []string{"url"}},
		{"an id that is not url-safe", "- {id: My Pad, url: https://pad.example.org, name: Pad}\n", []string{"id"}},
		{"an unknown key", "- {id: pad, url: https://pad.example.org, name: Pad, colour: red}\n", []string{"colour"}},
		{"broken yaml", "- {id: pad\n", []string{"services.yaml"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseServices("services.yaml", []byte(c.yaml))
			if err == nil {
				t.Fatal("accepted a services file that should have been refused")
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q does not mention %q", err, want)
				}
			}
		})
	}
}

// Preview images are measured at load so pages can carry width and height and
// never shift while loading. Anything unreadable is skipped, not fatal.
func TestMediaDimsSkipsWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	// a 1x1 png, the smallest real image
	png := []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0, 0, 0, 0x0d, 'I', 'H', 'D', 'R',
		0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0,
		0x1f, 0x15, 0xc4, 0x89,
		0, 0, 0, 0x0a, 'I', 'D', 'A', 'T',
		0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1,
		0x0d, 0x0a, 0x2d, 0xb4,
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	dims := mediaDims(dir)
	if got := dims["ok.png"]; got != [2]int{1, 1} {
		t.Errorf("ok.png = %v, want [1 1]", got)
	}
	if _, ok := dims["notes.txt"]; ok {
		t.Error("a text file was measured as an image")
	}

	// a directory that is not there at all is simply empty, never a failure
	if n := len(mediaDims(filepath.Join(dir, "absent"))); n != 0 {
		t.Errorf("a missing media dir returned %d entries", n)
	}
}

// The UI string chain: an operator override wins, then the locale's own table,
// then its base language, then English, and finally the key itself so a
// missing string is visible rather than blank.
func TestStrOverrideChain(t *testing.T) {
	cfg := &Config{Site: Site{
		Locales: []string{"fr", "en"},
		Strings: map[string]LString{
			"search.label": {"fr": "Trouver"},
			"nav.top":      {"": "Up"},
		},
	}}
	for _, c := range []struct{ locale, key, want string }{
		{"fr", "search.label", "Trouver"},                               // the override
		{"en", "search.label", "Search"},                                // no override for en, built-in wins
		{"fr", "nav.top", "Up"},                                         // a bare override serves every locale
		{"fr", "search.empty", "Aucun résultat. Essayez un autre mot."}, // built-in French
		{"pt-BR", "detail.back", "Voltar"},                              // regional finds its base table
		{"de", "detail.back", "Zurück"},                                 // a built-in language cairn ships
		{"xx", "detail.back", "Back"},                                   // unknown language falls to English
		{"fr", "no.such.key", "no.such.key"},                            // a missing key names itself
	} {
		if got := cfg.Str(c.locale, c.key); got != c.want {
			t.Errorf("Str(%q, %q) = %q, want %q", c.locale, c.key, got, c.want)
		}
	}
}

// categories.yaml is the one file whose only job is naming and ordering, so
// its refusals have to be as clear as the services ones.
//
// Note what is deliberately NOT refused: a category id is an anchor, not a URL
// path, so unlike a service id it may carry accents or capitals. The template
// percent-encodes it. Tightening this would break every site whose categories
// are named in its own language.
func TestParseCategoriesRefusals(t *testing.T) {
	for _, c := range []struct{ name, yaml, want string }{
		{"a mapping instead of a list", "photos:\n  name: Photos\n", "categories.yaml"},
		{"a category without an id", "- {name: Photos}\n", "id"},
		{"a duplicate id", "- {id: photos, name: A}\n- {id: photos, name: B}\n", "duplicate"},
		{"an unknown key", "- {id: photos, name: Photos, colour: red}\n", "colour"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseCategories("categories.yaml", []byte(c.yaml))
			if err == nil {
				t.Fatal("accepted a categories file that should have been refused")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("message %q does not mention %q", err, c.want)
			}
		})
	}
}

func TestCategoryIDsMayBeWrittenInAnyLanguage(t *testing.T) {
	cats, err := parseCategories("categories.yaml", []byte("- {id: Photothèque, name: Photos}\n"))
	if err != nil {
		t.Fatalf("a category id with an accent was refused: %v", err)
	}
	if len(cats) != 1 || cats[0].ID != "Photothèque" {
		t.Errorf("parsed %v, want the id kept verbatim", cats)
	}
}
