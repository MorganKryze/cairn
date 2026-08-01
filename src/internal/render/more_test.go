package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

const moreSite = "locales: [en, fr]\n"

func moreCards(t *testing.T, services string) map[string]string {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     moreSite,
		"services.yaml": services,
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

// The link to a detail page is a glyph in the card's corner rather than words
// at the end of the description. A glyph says nothing on its own, so the name
// a screen reader reads has to be carried by the attribute, and it has to name
// the service: a page full of "Learn more" tells someone using a link list
// nothing at all.
func TestTheDetailLinkIsNamedForTheServiceItOpens(t *testing.T) {
	pages := moreCards(t, "- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n")
	home := pages["en"]
	if !strings.Contains(home, `aria-label="Pad, Learn more"`) {
		t.Errorf("the detail link is not named for its service:\n%s", cardOf(home))
	}
	// The label is a UI string, so it follows the page rather than the service.
	if !strings.Contains(pages["fr"], `aria-label="Pad, En savoir plus"`) {
		t.Errorf("the french page does not name the link in french:\n%s", cardOf(pages["fr"]))
	}
	// title carries the same words for a pointer, which is the only hint
	// somebody who is not using a screen reader gets from a glyph.
	if !strings.Contains(home, `title="Learn more"`) {
		t.Error("the glyph has no tooltip")
	}
	// The svg is decoration: the attribute above is the name, and a glyph
	// announced twice is a glyph announced wrong.
	if !strings.Contains(home, `<svg class="more-glyph" viewBox="0 0 24 24"`) || !strings.Contains(cardOf(home), `aria-hidden="true"`) {
		t.Errorf("the glyph is not marked as decoration:\n%s", cardOf(home))
	}
}

// A service with nothing more to show gets no link, which is what makes the
// glyph mean something when it is there.
func TestACardWithNoDetailPageHasNoGlyph(t *testing.T) {
	pages := moreCards(t, "- {id: pad, url: https://pad.example.org, name: Pad, desc: Just a link.}\n")
	if strings.Contains(pages["en"], "card-more") {
		t.Errorf("a card with no detail page carries the link anyway:\n%s", cardOf(pages["en"]))
	}
}

// The description is prose now, with nothing appended to it. That matters for
// the language mark: a description that fell back to another language used to
// share its span with a label written in the page's language.
func TestTheDescriptionIsLeftAlone(t *testing.T) {
	pages := moreCards(t, "- {id: pad, url: https://pad.example.org, name: Pad, desc: Write together., details: More.}\n")
	if !strings.Contains(pages["en"], `<span class="card-desc">Write together.</span>`) {
		t.Errorf("the description is not a clean span:\n%s", cardOf(pages["en"]))
	}
}

// cardOf trims a page down to its first card, so a failure prints something a
// person can read instead of nine kilobytes.
func cardOf(page string) string {
	i := strings.Index(page, `<li class="card"`)
	if i < 0 {
		return page
	}
	j := strings.Index(page[i:], "</li>")
	if j < 0 {
		return page[i:]
	}
	return page[i : i+j+5]
}
