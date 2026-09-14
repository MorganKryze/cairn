package config

import (
	"strings"
	"testing"
)

// Every glyph a header link can name. A name that silently leaves the map
// turns a working config into a refused one on upgrade, so the set is pinned:
// removing one has to be a decision made here, not a side effect elsewhere.
func TestTheGlyphSet(t *testing.T) {
	want := []string{
		"announce", "book", "calendar", "chat", "cloud", "code", "coffee", "community",
		"download", "file", "github", "globe", "heart", "help", "home", "key",
		"legal", "location", "lock", "mail", "news", "photos", "portfolio", "privacy",
		"rss", "social", "status", "support", "user", "video",
	}
	got := GlyphNames()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("the glyph set is\n  %v\nwant\n  %v", got, want)
	}
	for _, n := range got {
		d := Glyphs[n]
		// Inner markup only: the page wraps it in the one <svg> every glyph
		// shares, so a pasted wrapper would nest and draw nothing.
		if strings.Contains(d, "<svg") || !strings.Contains(d, "<") {
			t.Errorf("glyph %q is not inner svg markup: %.60q", n, d)
		}
	}
}
