package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// pages renders a config dir and returns the home and the detail page.
func pages(t *testing.T, site string) (home, detail string) {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     site,
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, map[string]status.State{"pad": {Level: status.LevelUp}})
	if err != nil {
		t.Fatal(err)
	}
	return string(m.Pages["en"].HTML), string(m.Pages["en/pad"].HTML)
}

const withGatus = "locales: [en]\nstatus:\n  gatus: https://status.example.org\n"

// The same address written the generic way. Every page it produces has to be
// the page status.gatus produces, byte for byte: the second spelling exists so
// other monitors have a key to be named by, not so a Gatus site renders
// differently depending on which line the operator typed.
const withProvider = "locales: [en]\nstatus:\n  provider: gatus\n  url: https://status.example.org\n"

func TestNamingGatusChangesNothingItRenders(t *testing.T) {
	oldHome, oldDetail := pages(t, withGatus)
	newHome, newDetail := pages(t, withProvider)
	if oldHome != newHome {
		t.Error("the home page differs between status.gatus and status.provider: gatus")
	}
	if oldDetail != newDetail {
		t.Error("the detail page differs between status.gatus and status.provider: gatus")
	}
	// The pill is the part that would go missing, since everything about it
	// keys off "is there an address", so it is worth naming on its own.
	if !strings.Contains(newHome, `<a class="status-pill status-up" href="https://status.example.org/endpoints/_pad"`) {
		t.Error("a site that names its provider draws no pill")
	}
}

// By default a pill leads to its endpoint on the status page, on the card and
// on the detail page alike.
func TestStatusPillLinksByDefault(t *testing.T) {
	home, detail := pages(t, withGatus)
	for _, page := range []struct{ name, html string }{{"home", home}, {"detail", detail}} {
		if !strings.Contains(page.html, `<a class="status-pill status-up" href="https://status.example.org/endpoints/_pad"`) {
			t.Errorf("%s: no linked pill", page.name)
		}
		// A link read out of its card has to name what it points at.
		if !strings.Contains(page.html, `aria-label="Pad, Online, view status"`) {
			t.Errorf("%s: pill link does not name the service and its target", page.name)
		}
	}
}

// Kuma publishes one status page for a whole set of monitors and has no page
// per monitor, so there is no endpoint path to build and no key to build it
// from. A pill that kept the Gatus shape would point at a page that does not
// exist, which is the bug v1.13.2 fixed for Gatus itself.
func TestAKumaPillLinksToTheStatusPage(t *testing.T) {
	const site = "locales: [en]\nstatus:\n  provider: kuma\n  url: https://kuma.example.org\n  slug: tools\n"
	home, detail := pages(t, site)
	for _, page := range []struct{ name, html string }{{"home", home}, {"detail", detail}} {
		// The slug is what Kuma itself puts in the URL of a published page, so
		// the address and the slug are enough: no second key to keep in step.
		if !strings.Contains(page.html, `href="https://kuma.example.org/status/tools"`) {
			t.Errorf("%s: the pill does not link to the published status page", page.name)
		}
		if strings.Contains(page.html, "/endpoints/") {
			t.Errorf("%s: the pill kept the gatus endpoint path, which kuma does not serve", page.name)
		}
	}
	// status.page still wins, the way it does for Gatus: only the operator
	// knows whether the address cairn polls is one a visitor can reach.
	public, _ := pages(t, site+"  page: https://status.example.org/x\n")
	if !strings.Contains(public, `href="https://status.example.org/x"`) {
		t.Error("status.page did not override the derived one")
	}
}

// Four monitors of the eight surveyed distinguish "works, but not well" from
// "does not work", and as many distinguish "off on purpose" from "broken".
// Folding either into down tells a visitor the opposite of what their monitor
// went to the trouble of saying.
func TestDegradedAndMaintenanceAreTheirOwnPills(t *testing.T) {
	for _, c := range []struct{ level, class, label string }{
		{status.LevelDegraded, "status-degraded", "Degraded"},
		{status.LevelMaintenance, "status-maintenance", "Maintenance"},
	} {
		t.Run(c.level, func(t *testing.T) {
			cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
				"site.yaml":     withGatus,
				"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
			}))
			if err != nil {
				t.Fatal(err)
			}
			m, err := BuildModel(cfg, map[string]status.State{"pad": {Level: c.level}})
			if err != nil {
				t.Fatal(err)
			}
			home := string(m.Pages["en"].HTML)
			if !strings.Contains(home, `class="status-pill `+c.class+`"`) {
				t.Errorf("%s has no pill of its own", c.level)
			}
			// The label is what carries the meaning for someone who cannot tell
			// the colours apart, so a pill that painted itself but did not name
			// itself would pass the class check and fail the visitor.
			if !strings.Contains(home, `aria-label="Pad, `+c.label+`, view status"`) {
				t.Errorf("%s has no label a visitor can read", c.level)
			}
			for _, other := range []string{"status-up", "status-down", "status-unknown"} {
				if strings.Contains(home, other) {
					t.Errorf("%s also rendered as %s", c.level, other)
				}
			}
		})
	}
}

// The pages are static, so a tab left open holds whatever Gatus said when it
// was opened. status.js swaps the pills in place; for that it needs a slot per
// service that is in the markup whether or not there is a pill in it, since
// the state it has to reach includes "no pill at all".
func TestEveryServiceHasAPillSlot(t *testing.T) {
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     withGatus,
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n- {id: wiki, url: /wiki, name: Wiki}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	// Gatus has answered and monitors pad only: wiki ends up with no pill.
	m, err := BuildModel(cfg, map[string]status.State{"pad": {Level: status.LevelUp}})
	if err != nil {
		t.Fatal(err)
	}
	home := string(m.Pages["en"].HTML)
	if !strings.Contains(home, `<span class="status-slot" data-status="pad"><a class="status-pill status-up"`) {
		t.Error("the pill is not in a slot naming its service")
	}
	if !strings.Contains(home, `<span class="status-slot" data-status="wiki"></span>`) {
		t.Error("a service gatus does not monitor has no slot, so a pill can never appear there")
	}
	if !strings.Contains(string(m.Pages["en/pad"].HTML), `<span class="status-slot" data-status="pad">`) {
		t.Error("the detail page pill is not in a slot")
	}
}

// Without a Gatus there is nothing to refresh, so none of it ships: no slot,
// no script, and no hole in the CSP for a request the page never makes.
func TestNoGatusShipsNoRefresh(t *testing.T) {
	home, _ := pages(t, "locales: [en]\n")
	for _, unwanted := range []string{"status-slot", "status.js"} {
		if strings.Contains(home, unwanted) {
			t.Errorf("a site without a status page still carries %q", unwanted)
		}
	}
	// The header, not the markup: default-src stays 'none' with nothing added
	// back for a request no script on the page is there to make.
	if csp := BuildCSP(mustLoad(t, "locales: [en]\n")); strings.Contains(csp, "connect-src") {
		t.Errorf("the CSP opens connect-src on a site with no status page: %s", csp)
	}
}

// The script polls cairn at the interval cairn polls Gatus: asking more often
// than the server can learn anything new is pure noise.
func TestRefreshScriptCarriesThePollInterval(t *testing.T) {
	home, detail := pages(t, withGatus+"  interval: 30s\n")
	for _, page := range []struct{ name, html string }{{"home", home}, {"detail", detail}} {
		if !strings.Contains(page.html, `<script src="`+AssetURL("status.js")+`" defer data-poll="30"></script>`) {
			t.Errorf("%s: no refresh script, or it does not carry the interval", page.name)
		}
	}
	// default-src is 'none', so the fetch it makes needs saying out loud.
	if !strings.Contains(BuildCSP(mustLoad(t, withGatus)), "connect-src 'self'") {
		t.Error("the CSP forbids the request status.js exists to make")
	}
}

func mustLoad(t *testing.T, site string) *config.Config {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     site,
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// status.linked: false keeps the state and drops everything that made the
// pill a control: the anchor, the target, and the label that announced a
// destination there no longer is.
func TestStatusLinkedFalseLeavesOnlyTheState(t *testing.T) {
	home, detail := pages(t, withGatus+"  linked: false\n")
	for _, page := range []struct{ name, html string }{{"home", home}, {"detail", detail}} {
		if !strings.Contains(page.html, `<span class="status-pill status-up"><span class="dot" aria-hidden="true"></span>Online</span>`) {
			t.Errorf("%s: the pill is not a plain span stating the state", page.name)
		}
		if strings.Contains(page.html, "a class=\"status-pill") {
			t.Errorf("%s: a pill is still a link", page.name)
		}
		if strings.Contains(page.html, "status.example.org/endpoints") {
			t.Errorf("%s: the status page URL is still in the markup", page.name)
		}
		// The screen-reader label promised a link; without one it would lie.
		if strings.Contains(page.html, "view status") {
			t.Errorf("%s: the pill still announces a destination", page.name)
		}
	}
}

// The option changes only the pill. The state still comes from Gatus, the
// card still opens the service, and a service Gatus does not monitor still
// gets no pill at all.
func TestStatusLinkedFalseChangesNothingElse(t *testing.T) {
	home, _ := pages(t, withGatus+"  linked: false\n")
	if !strings.Contains(home, "status-up") {
		t.Error("the state itself is gone")
	}
	if !strings.Contains(home, `href="https://pad.example.org"`) {
		t.Error("the card no longer links to its service")
	}

	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     withGatus + "  linked: false\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	// Gatus has answered, and knows nothing about this service.
	m, err := BuildModel(cfg, map[string]status.State{"elsewhere": {Level: status.LevelUp}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(m.Pages["en"].HTML), "status-pill") {
		t.Error("an unmonitored service grew a pill")
	}
}

// Without a Gatus there is no state to show, so the option is inert.
func TestStatusLinkedIsInertWithoutGatus(t *testing.T) {
	home, _ := pages(t, "locales: [en]\nstatus:\n  linked: false\n")
	if strings.Contains(home, "status-pill") {
		t.Error("a pill appeared without a Gatus configured")
	}
}

// The default has to survive an explicit true and an absent key alike.
func TestStatusLinkedDefaultsToLinked(t *testing.T) {
	for _, c := range []struct {
		name, yaml string
		want       bool
	}{
		{"absent", "", true},
		{"true", "linked: true\n", true},
		{"false", "linked: false\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
				"site.yaml":     "locales: [en]\nstatus:\n  gatus: https://status.example.org\n  " + c.yaml,
				"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
			}))
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.StatusLinked(); got != c.want {
				t.Errorf("StatusLinked() = %v, want %v", got, c.want)
			}
		})
	}
}
