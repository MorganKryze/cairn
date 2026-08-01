package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func flagPages(t *testing.T, site string) map[string]string {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml": site,
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, selfhosted: true}\n" +
			"- {id: mail, url: https://mail.example.org, name: Mail, selfhosted: false}\n",
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

const flagSite = "locales: [en, fr]\npages:\n  - id: hosting\n    title: {en: How we host, fr: Comment nous hébergeons}\n    body: {en: Text., fr: Texte.}\n"

// A page id resolves to the page in the language the visitor is reading, the
// way cairn's own footer links already do. Writing /en/hosting/ in the config
// would pin one language for everybody, which is the whole reason the id is
// what the key takes.
func TestTheFlagLinksToThePageInTheReadersLanguage(t *testing.T) {
	pages := flagPages(t, flagSite+"hosting_flag:\n  self: hosting\n")
	if !strings.Contains(pages["en"], `<a class="flag flag-self" href="/en/hosting/"`) {
		t.Errorf("the english page does not link the flag to the english page:\n%s", flagOf(pages["en"]))
	}
	if !strings.Contains(pages["fr"], `<a class="flag flag-self" href="/fr/hosting/"`) {
		t.Errorf("the french page does not link the flag to the french page:\n%s", flagOf(pages["fr"]))
	}
	// Only the flag that was given a target becomes a link. The other stays
	// what it has always been, so nothing moves for a site that sets one key.
	if !strings.Contains(pages["en"], `<span class="flag flag-external">`) {
		t.Errorf("the external flag became something else:\n%s", pages["en"])
	}
}

// A URL and an absolute path pass through untouched, for an explanation that
// lives outside cairn or behind the proxy in front of it.
func TestTheFlagAlsoTakesAURLOrAPath(t *testing.T) {
	pages := flagPages(t, "locales: [en]\nhosting_flag:\n  self: /why-self-hosted\n  external: https://example.org/why\n")
	for _, want := range []string{
		`<a class="flag flag-self" href="/why-self-hosted"`,
		`<a class="flag flag-external" href="https://example.org/why"`,
	} {
		if !strings.Contains(pages["en"], want) {
			t.Errorf("missing %s in:\n%s", want, pages["en"])
		}
	}
}

// Without the key, the flag is exactly what it was: a span, and a click on it
// opens the service like the rest of the card.
func TestWithoutTheKeyTheFlagIsStillASpan(t *testing.T) {
	pages := flagPages(t, "locales: [en]\n")
	if strings.Contains(pages["en"], `<a class="flag`) {
		t.Errorf("a site that set nothing got a link:\n%s", flagOf(pages["en"]))
	}
	if !strings.Contains(pages["en"], `<span class="flag flag-self">`) {
		t.Errorf("the flag is not a span any more:\n%s", flagOf(pages["en"]))
	}
}

// An external target opens where an external link opens, and carries the same
// rel the status pill does. An internal one does not: it is this site.
func TestAnExternalTargetOpensLikeAnExternalLink(t *testing.T) {
	pages := flagPages(t, flagSite+"hosting_flag:\n  self: hosting\n  external: https://example.org/why\n")
	if !strings.Contains(pages["en"], `href="https://example.org/why" target="_blank" rel="noopener noreferrer"`) {
		t.Errorf("the external target does not open in a new tab:\n%s", flagOf(pages["en"]))
	}
	if strings.Contains(pages["en"], `href="/en/hosting/" target=`) {
		t.Error("a page cairn serves itself should open in place")
	}
}

func flagOf(page string) string {
	i := strings.Index(page, `class="flag`)
	if i < 0 {
		return "(no flag)"
	}
	start := strings.LastIndex(page[:i], "<")
	j := strings.Index(page[i:], "</span>")
	return page[start : i+j+7]
}
