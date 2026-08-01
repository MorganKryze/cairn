package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func backPages(t *testing.T) map[string]string {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en, ar]\npages:\n  - id: legal\n    title: {en: Legal, ar: قانوني}\n    body: {en: Text., ar: نص.}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, map[string]status.State{})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, p := range m.Pages {
		out[k] = string(p.HTML)
	}
	return out
}

// The arrow used to be a CSS ::before, which some screen readers read out as
// "left arrow" before the word, and which a stylesheet that fails to load
// takes with it. It is an inline svg now, marked as decoration, so the word is
// the whole message either way.
func TestTheBackArrowIsDecorationInTheMarkup(t *testing.T) {
	pages := backPages(t)
	for _, name := range []string{"en/pad", "en/legal"} {
		page := pages[name]
		if !strings.Contains(page, `<a class="back" href="/en/"><svg class="back-glyph"`) {
			t.Errorf("%s: the back link carries no glyph of its own", name)
		}
		if !strings.Contains(page, `aria-hidden="true"`) {
			t.Errorf("%s: the glyph is not marked as decoration", name)
		}
		// The word stays, and it stays translated.
		if !strings.Contains(page, "Back</a>") {
			t.Errorf("%s: the back link lost its label", name)
		}
	}
	// Detail and simple pages carry the same control, so a visitor learns it
	// once. The two templates had drifted before over smaller things.
	if a, b := backOf(pages["en/pad"]), backOf(pages["en/legal"]); a != b {
		t.Errorf("the two pages disagree on the back link:\n%s\n%s", a, b)
	}
}

// A right-to-left page reads the other way round, so the arrow has to point
// the other way. It was a different character before; now the same glyph is
// mirrored, which is the stylesheet's job rather than the template's.
func TestTheBackArrowTurnsAroundInRTL(t *testing.T) {
	pages := backPages(t)
	if !strings.Contains(pages["ar/legal"], `<a class="back" href="/ar/"><svg class="back-glyph"`) {
		t.Error("the arabic page has no back glyph")
	}
	if !strings.Contains(pages["ar/legal"], `dir="rtl"`) {
		t.Error("the arabic page is not marked right to left, so nothing would mirror")
	}
}

func backOf(page string) string {
	i := strings.Index(page, `<a class="back"`)
	if i < 0 {
		return "(no back link)"
	}
	j := strings.Index(page[i:], "</a>")
	return page[i : i+j+4]
}
