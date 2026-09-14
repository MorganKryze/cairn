package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// sign_in puts a lock after the name and a note on the detail page. Three
// things about it are easy to get wrong and invisible once wrong: the lock
// has to stay with the last word, its name has to reach a screen reader
// without becoming part of the name search.js reads, and a service that does
// not set it has to render exactly what it did before.
func TestSignIn(t *testing.T) {
	build := func(services string) map[string]string {
		cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
			"services.yaml": services,
			"site.yaml":     "locales: [en, fr]\n",
		}))
		if err != nil {
			t.Fatal(err)
		}
		m, err := BuildModel(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for k, p := range m.Pages {
			out[k] = string(p.HTML)
		}
		return out
	}
	const svc = "- {id: pad, url: https://pad.example.org, name: Shared notepad, category: tools, details: {en: A page.}"

	off := build(svc + "}\n")
	for _, k := range []string{"en", "en/pad"} {
		for _, gone := range []string{"card-lock", "name-end", `class="signin"`} {
			if strings.Contains(off[k], gone) {
				t.Errorf("a service without sign_in carries %s on %s", gone, k)
			}
		}
	}

	on := build(svc + ", sign_in: true}\n")
	// The last word and the lock in one span, the rest of the name before it.
	if !strings.Contains(on["en"], `Shared <span class="name-end">notepad<span class="card-lock" title="Sign-in required">`) {
		t.Error("the lock is not bound to the name's last word")
	}
	if !strings.Contains(on["en"], `role="img" aria-label="Sign-in required"`) {
		t.Error("the lock has no name a screen reader can read")
	}
	// search.js matches on the name's text, so the lock must add none to it.
	name := regexp.MustCompile(`(?s)<a class="card-name"[^>]*>(.*?)</a>`).FindStringSubmatch(on["en"])
	if name == nil {
		t.Fatal("no card name on the home page")
	}
	if text := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(name[1], ""); text != "Shared notepad" {
		t.Errorf("the name reads %q to anything that takes its text", text)
	}
	if !strings.Contains(on["en/pad"], `class="signin" role="note"`) ||
		!strings.Contains(on["en/pad"], "You will need an account to use this.") {
		t.Error("the detail page does not carry the note")
	}
	if !strings.Contains(on["fr/pad"], "<strong>Connexion requise</strong>") {
		t.Error("the note is not in the reader's language")
	}

	// A one-word name is all tail, and still binds.
	one := build("- {id: pad, url: https://pad.example.org, name: Pad, category: tools, sign_in: true}\n")
	if !strings.Contains(one["en"], `<span class="name-end">Pad<span class="card-lock"`) {
		t.Error("a one-word name does not bind to the lock")
	}
}
