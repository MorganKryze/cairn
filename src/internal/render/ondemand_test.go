package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// Whatever the monitor says about an on-demand service is about a nap, not an
// outage, so neither the card nor the detail page carries a pill or a slot a
// live update could fill.
func TestAnOnDemandServiceIgnoresItsMonitor(t *testing.T) {
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nstatus:\n  gatus: https://status.example.org\n",
		"services.yaml": "- {id: nap, url: https://nap.example.org, name: Nap, details: More., on_demand: true}\n" +
			"- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{status.LevelUp, status.LevelDegraded, status.LevelMaintenance, status.LevelDown, ""} {
		statuses := map[string]status.State{"pad": {Level: status.LevelUp}}
		if level != "" {
			statuses["nap"] = status.State{Level: level}
		}
		m, err := BuildModel(cfg, statuses)
		if err != nil {
			t.Fatal(err)
		}
		home := string(m.Pages["en"].HTML)
		nap := string(m.Pages["en/nap"].HTML)
		if strings.Contains(home, `data-status="nap"`) {
			t.Errorf("monitor says %q: the on-demand card has a status slot", level)
		}
		if strings.Contains(nap, `data-status="nap"`) || strings.Contains(nap, "status-pill") {
			t.Errorf("monitor says %q: the on-demand detail page has a pill", level)
		}
		if !strings.Contains(home, `data-status="pad"`) || !strings.Contains(home, "status-pill status-up") {
			t.Errorf("monitor says %q: the plain neighbour lost its pill", level)
		}
	}
}

// pagesOf builds a site from its two files and returns every page as text.
func pagesOf(t *testing.T, site, services string) map[string]string {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{"site.yaml": site, "services.yaml": services}))
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

// footOf is the card foot of the one card on a home page.
func footOf(t *testing.T, home string) string {
	t.Helper()
	foot := regexp.MustCompile(`(?s)<div class="card-foot">.*?</div>`).FindString(home)
	if foot == "" {
		t.Fatal("the card has no foot")
	}
	return foot
}

const sleeper = "- {id: nap, url: https://nap.example.org, name: Nap, selfhosted: true, details: More., on_demand: true}\n"

// The slow first load is true with or without a monitor, so the mark sits where
// the pill goes on both kinds of site, and it is a word, not a dot.
func TestTheOnDemandMarkTakesThePillsPlace(t *testing.T) {
	for name, site := range map[string]string{
		"monitor":    "locales: [en]\nstatus:\n  gatus: https://status.example.org\n",
		"no monitor": "locales: [en]\n",
	} {
		foot := footOf(t, pagesOf(t, site, sleeper)["en"])
		if !strings.HasSuffix(foot, `<span class="ondemand-mark">On demand</span></div>`) {
			t.Errorf("%s: the foot does not end on the mark: %s", name, foot)
		}
		if strings.Contains(foot, "dot") {
			t.Errorf("%s: the mark carries a dot: %s", name, foot)
		}
	}
	// Without a flag the foot holds the mark alone, and still exists.
	foot := footOf(t, pagesOf(t, "locales: [en]\n", "- {id: nap, url: https://nap.example.org, name: Nap, on_demand: true}\n")["en"])
	if foot != `<div class="card-foot"><span class="ondemand-mark">On demand</span></div>` {
		t.Errorf("a flagless on-demand card's foot is %s", foot)
	}
}

// A screen reader reads the row as "On demand: Starts when opened...", and a
// declared state keeps its line under the row as it does under a pill.
func TestTheDetailPageSaysItStartsWhenOpened(t *testing.T) {
	const row = `<div class="detail-state-row"><span class="ondemand-mark">On demand</span><span class="visually-hidden">: </span><span class="ondemand-note">Starts when opened: the first load may take a few seconds.</span></div>`
	for name, site := range map[string]string{
		"monitor":    "locales: [en]\nstatus:\n  gatus: https://status.example.org\n",
		"no monitor": "locales: [en]\n",
	} {
		if p := pagesOf(t, site, sleeper)["en/nap"]; !strings.Contains(p, row) {
			t.Errorf("%s: the detail page has no mark and note row", name)
		}
	}
	beta := pagesOf(t, "locales: [en]\n", strings.Replace(sleeper, "on_demand: true", "on_demand: true, state: beta", 1))["en/nap"]
	if !strings.Contains(beta, row+`<p class="detail-note"><span class="detail-state detail-state-beta">`) {
		t.Error("the beta line does not follow the on-demand row")
	}
}

// strings rewords the mark and the note per locale; a locale it leaves out
// keeps the built-in words.
func TestAStringsOverrideRewordsTheMark(t *testing.T) {
	p := pagesOf(t,
		"locales: [en, fr]\nstrings: {ondemand.label: {fr: \"Sur réveil\"}, ondemand.note: {fr: \"Se réveille.\"}}\n",
		sleeper)
	for k, want := range map[string]string{
		"fr":     `<span class="ondemand-mark">Sur réveil</span>`,
		"fr/nap": `<span class="ondemand-note">Se réveille.</span>`,
		"en":     `<span class="ondemand-mark">On demand</span>`,
		"en/nap": `<span class="ondemand-note">Starts when opened: the first load may take a few seconds.</span>`,
	} {
		if !strings.Contains(p[k], want) {
			t.Errorf("%s does not carry %s", k, want)
		}
	}
}

// The note lives on the detail page only, so a card with nothing else to show
// still has to lead there.
func TestAnOnDemandCardAlwaysLeadsToItsPage(t *testing.T) {
	home := pagesOf(t, "locales: [en]\n", "- {id: nap, url: https://nap.example.org, name: Nap, on_demand: true}\n")["en"]
	if !strings.Contains(home, `<a class="card-more" href="/en/nap/"`) {
		t.Error("an on-demand card without details has no info button")
	}
}

// soon and retired already take the link and the pill away; on_demand beside
// them adds nothing to either page.
func TestADisablingStateHidesTheMark(t *testing.T) {
	p := pagesOf(t, "locales: [en]\n", strings.Replace(sleeper, "on_demand: true", "on_demand: true, state: retired", 1))
	for _, k := range []string{"en", "en/nap"} {
		if strings.Contains(p[k], "ondemand-") {
			t.Errorf("a retired service still carries the mark or the note on %s", k)
		}
	}
}
