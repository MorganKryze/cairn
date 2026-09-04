package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// site.search removes the box, and it has to remove all of it. The live one is
// on the home page, a disabled copy is on every other page for layout parity,
// and search.js binds Cmd+K: leaving any of the three behind would give a
// shortcut that focuses nothing, or a bar an operator asked to be rid of.
func TestSearchCanBeTurnedOff(t *testing.T) {
	build := func(site string) map[string]string {
		files := map[string]string{
			"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, category: tools, details: {en: A page.}}\n",
		}
		if site != "" {
			files["site.yaml"] = site
		}
		cfg, err := config.Load(testutil.WriteFiles(t, files))
		if err != nil {
			t.Fatal(err)
		}
		m, err := BuildModel(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]string{
			"home":   string(m.Pages["en"].HTML),
			"detail": string(m.Pages["en/pad"].HTML),
		}
	}

	// Unset is on: the key is opt-out, so every site that predates it keeps the
	// box it has.
	for where, h := range build("") {
		for _, want := range []string{`id="search"`, "/static/search."} {
			if !strings.Contains(h, want) && where == "home" {
				t.Errorf("with no site.search the %s page lost %s", where, want)
			}
		}
	}
	on := build("")
	if !strings.Contains(on["detail"], `class="search" aria-hidden="true"`) {
		t.Error("the detail page lost the disabled copy that keeps the header the same shape")
	}

	off := build("search: false\n")
	for where, h := range off {
		for _, gone := range []string{`class="search"`, `id="search"`, "/static/search.", "search-kbd"} {
			if strings.Contains(h, gone) {
				t.Errorf("search: false left %s on the %s page", gone, where)
			}
		}
	}
	// The row is still emitted, and the stylesheet is what hides it: asserting
	// the div is gone would pin a shape the template does not have.
	if !strings.Contains(off["home"], `class="wrap menu"`) {
		t.Error("the menu row itself went away; the CSS is what collapses it")
	}
}
