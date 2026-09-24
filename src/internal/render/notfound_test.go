package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// The 404 page answers for every address that leads nowhere. A canonical link
// would name one of them, and a search engine indexing it would list a page
// that says there is nothing here.
func TestThe404PageClaimsNoAddress(t *testing.T) {
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [en, fr]\n" +
			"strings: {notfound.body: {fr: \"Rien ici. [Écrivez-nous](mailto:hi@example.org).\"}}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{"en", "fr"} {
		page := string(m.NotFound[loc].HTML)
		if page == "" {
			t.Fatalf("no 404 page for %s", loc)
		}
		for _, bad := range []string{`rel="canonical"`, `hreflang=`, `og:url`} {
			if strings.Contains(page, bad) {
				t.Errorf("%s: the 404 page carries %s", loc, bad)
			}
		}
		if !strings.Contains(page, `<meta name="robots" content="noindex">`) {
			t.Errorf("%s: the 404 page does not ask to stay out of the index", loc)
		}
	}
	// The operator's wording goes through markdown like any other body text.
	if fr := string(m.NotFound["fr"].HTML); !strings.Contains(fr, `<a href="mailto:hi@example.org">Écrivez-nous</a>`) {
		t.Errorf("the operator's override did not reach the French page:\n%s", fr)
	}
}
