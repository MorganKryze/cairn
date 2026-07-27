package main

import (
	"strings"
	"testing"
)

func TestLocaleDir(t *testing.T) {
	for _, c := range []struct{ locale, want string }{
		{"en", "ltr"}, {"fr", "ltr"}, {"pt-BR", "ltr"},
		{"ar", "rtl"}, {"he", "rtl"}, {"fa", "rtl"}, {"ar-EG", "rtl"}, {"AR", "rtl"},
	} {
		if got := localeDir(c.locale); got != c.want {
			t.Errorf("localeDir(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

// A site whose content is Arabic gets a right-to-left page; the layout itself
// follows from the logical CSS properties, so only dir has to be emitted.
func TestRTLPageCarriesDirection(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [ar, en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	pages := current.Load().Pages
	if got := string(pages["ar"].HTML); !strings.Contains(got, `<html lang="ar" dir="rtl"`) {
		t.Error("the Arabic page is missing dir=rtl")
	}
	if got := string(pages["en"].HTML); !strings.Contains(got, `<html lang="en" dir="ltr"`) {
		t.Error("the English page is missing dir=ltr")
	}
}

// Physical left/right rules do not mirror; the stylesheet has to stay logical
// or a right-to-left page comes out with its spacing on the wrong side.
func TestStylesheetStaysDirectionAgnostic(t *testing.T) {
	css, err := embedded.ReadFile("assets/style.css")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(css), "\n") {
		trimmed := strings.TrimSpace(line)
		// the two arrow glyphs are flipped on purpose under [dir="rtl"]
		if strings.Contains(trimmed, `[dir="rtl"]`) {
			continue
		}
		for _, bad := range []string{"margin-left:", "margin-right:", "padding-left:", "padding-right:"} {
			if strings.Contains(trimmed, bad) {
				t.Errorf("style.css:%d uses %s, want its -inline- equivalent: %s", i+1, bad, trimmed)
			}
		}
		for _, bad := range []string{"left:", "right:"} {
			if strings.HasPrefix(trimmed, bad) || strings.Contains(trimmed, "{ "+bad) || strings.Contains(trimmed, "; "+bad) {
				t.Errorf("style.css:%d positions with %s, want inset-inline-*: %s", i+1, bad, trimmed)
			}
		}
	}
}
