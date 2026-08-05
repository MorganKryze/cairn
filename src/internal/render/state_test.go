package render_test

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// statePages renders a site with one service per family, plus a status address
// so the slot exists for the services that are allowed one.
func statePages(t *testing.T, site string) map[string]string {
	t.Helper()
	if site == "" {
		site = "locales: [en]\nstatus:\n  gatus: https://status.example.org\n"
	}
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml": site,
		"services.yaml": "- {id: plain, url: 'https://plain.example.org', name: Plain}\n" +
			"- {id: beta, url: 'https://beta.example.org', name: Beta one, state: beta}\n" +
			"- {id: soon, name: Soon one, state: soon}\n" +
			"- {id: gone, url: 'https://gone.example.org', name: Gone one, state: retired}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, map[string]status.State{
		"beta": {Level: status.LevelUp}, "plain": {Level: status.LevelUp},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, p := range m.Pages {
		out[k] = string(p.HTML)
	}
	return out
}

func TestTheBadgeIsTheCardsFirstChild(t *testing.T) {
	en := statePages(t, "")["en"]
	// Being read before the name is the whole point of the placement, and the
	// order in the markup is what a screen reader follows. Compared inside one
	// card: across the page the first card-main belongs to the card with no
	// badge at all, which made the first version of this pass on nothing.
	cards := strings.Split(en, `<li class="card`)
	seen := 0
	for _, c := range cards {
		i := strings.Index(c, `class="card-badge`)
		if i < 0 {
			continue
		}
		seen++
		j := strings.Index(c, `class="card-main"`)
		if j < 0 {
			t.Fatal("a card carries a badge and no body")
		}
		if i > j {
			t.Errorf("the badge is written after the card body, so it is read after the name")
		}
	}
	if seen != 3 {
		t.Errorf("%d cards carry a badge, want 3", seen)
	}
}

func TestADisablingStateRendersNoAnchor(t *testing.T) {
	en := statePages(t, "")["en"]
	if strings.Contains(en, "https://gone.example.org") {
		t.Error("a retired service still puts its address in the page")
	}
	if !strings.Contains(en, `<span class="card-name">Gone one</span>`) {
		t.Error("the name of a retired service is still an anchor")
	}
}

func TestADecorativeStateKeepsTheAnchor(t *testing.T) {
	en := statePages(t, "")["en"]
	if !strings.Contains(en, `href="https://beta.example.org"`) {
		t.Error("a beta service lost its link")
	}
}

func TestADecorativeStateKeepsTheStatusPill(t *testing.T) {
	en := statePages(t, "")["en"]
	if !strings.Contains(en, `data-status="beta"`) {
		t.Error("a beta service lost its status slot: it is live and polled")
	}
}

func TestADisablingStateGetsNoStatusSlot(t *testing.T) {
	en := statePages(t, "")["en"]
	if strings.Contains(en, `data-status="soon"`) || strings.Contains(en, `data-status="gone"`) {
		t.Error("a disabled service kept a status slot, so a pill can still be swapped into it")
	}
}

func TestTheBadgeIsLocalised(t *testing.T) {
	pages := statePages(t, "locales: [en, fr]\nstatus:\n  gatus: https://status.example.org\n")
	if !strings.Contains(pages["en"], "Coming soon") {
		t.Error("the English page does not say Coming soon")
	}
	if !strings.Contains(pages["fr"], "Bientôt disponible") {
		t.Error("the French page does not say Bientôt disponible")
	}
}

func TestADisabledCardIsStillSearchable(t *testing.T) {
	en := statePages(t, "")["en"]
	if !strings.Contains(en, "Gone one") {
		t.Error("a retired service left the page entirely: it is still information")
	}
}
