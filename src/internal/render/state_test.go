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

// detailPage renders one service's detail page in the given locale, with a
// status address so a live service has a pill to sit under.
func detailPage(t *testing.T, locales, service, page string) string {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: " + locales + "\nstatus:\n  gatus: https://status.example.org\n",
		"services.yaml": service,
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, map[string]status.State{"beta": {Level: status.LevelUp}})
	if err != nil {
		t.Fatal(err)
	}
	return string(m.Pages[page].HTML)
}

const betaService = "- {id: beta, url: 'https://beta.example.org', name: Beta one, state: beta, details: Long.}\n"

func TestTheDetailPageSaysWhatTheStateMeansUnderThePill(t *testing.T) {
	page := detailPage(t, "[en]", betaService, "en/beta")
	// One paragraph, so a screen reader reads the word and its meaning as one
	// item, and the hidden colon so it does not run them together as one phrase.
	want := `<p class="detail-note"><span class="detail-state detail-state-beta">Beta</span>` +
		`<span class="visually-hidden">: </span><span>Available, but still being tested: it may change.</span></p>`
	if !strings.Contains(page, want) {
		t.Fatalf("the detail page does not carry the state line %s", want)
	}
	row := strings.Index(page, `<div class="detail-state-row">`)
	note := strings.Index(page, `<p class="detail-note">`)
	if row < 0 {
		t.Fatal("a live service lost the row its pill sits in")
	}
	if note < row {
		t.Error("the state line is written before the pill's row, so it is read before what the monitor measured")
	}
	if strings.Contains(page[row:note], `class="detail-state `) {
		t.Error("the state word is still in the pill's row as well as on its own line")
	}
}

func TestAStateNoteReplacesTheBuiltInOne(t *testing.T) {
	page := detailPage(t, "[en]",
		"- {id: beta, url: 'https://beta.example.org', name: Beta one, state: beta, state_note: Ask Sam before relying on it., details: Long.}\n",
		"en/beta")
	if !strings.Contains(page, `<span class="visually-hidden">: </span><span>Ask Sam before relying on it.</span></p>`) {
		t.Error("the service's own state_note is not the line under the pill")
	}
	if strings.Contains(page, "still being tested") {
		t.Error("the built-in note is still on a page whose service replaced it")
	}
}

func TestAStateNoteInAnotherLanguageIsMarkedAsSuch(t *testing.T) {
	page := detailPage(t, "[en, fr]",
		"- {id: beta, url: 'https://beta.example.org', name: Beta one, state: beta, state_note: {en: Ask Sam first.}, details: Long.}\n",
		"fr/beta")
	if !strings.Contains(page, `<span lang="en">Ask Sam first.</span></p>`) {
		t.Error("an English state_note on the French page is not marked lang=\"en\", so a screen reader reads it in French")
	}
}

func TestAPageWithoutStateHasNoStateLine(t *testing.T) {
	page := detailPage(t, "[en]", "- {id: plain, url: 'https://plain.example.org', name: Plain, details: Long.}\n", "en/plain")
	if page == "" {
		t.Fatal("no detail page for the plain service")
	}
	if strings.Contains(page, "detail-note") {
		t.Error("a service with no state still prints a state line")
	}
}

func TestARetiredPageHasNoEmptyPillRow(t *testing.T) {
	page := detailPage(t, "[en]", "- {id: gone, url: 'https://gone.example.org', name: Gone one, state: retired, details: Moved.}\n", "en/gone")
	if strings.Contains(page, "detail-state-row") {
		t.Error("a retired service, which has no pill, still prints the pill's row")
	}
	if !strings.Contains(page, `<p class="detail-note"><span class="detail-state detail-state-retired">`) {
		t.Error("a retired service lost its state line, the one place that says it is gone")
	}
}

func TestADisabledDetailPageHasNoOpenButton(t *testing.T) {
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: gone, url: 'https://gone.example.org', name: Gone one, state: retired, details: Moved.}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := string(m.Pages["en/gone"].HTML)
	if strings.Contains(page, `class="btn`) {
		t.Error("a retired service still offers a button, which is a promise not kept")
	}
	if strings.Contains(page, "https://gone.example.org") {
		t.Error("the dead address is still in the page")
	}
	if !strings.Contains(page, "Moved.") {
		t.Error("the prose is gone, and that page is the only place left to say where to go instead")
	}
}
