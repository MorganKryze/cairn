package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
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
	m, err := BuildModel(cfg, map[string]bool{"pad": true})
	if err != nil {
		t.Fatal(err)
	}
	return string(m.Pages["en"].HTML), string(m.Pages["en/pad"].HTML)
}

const withGatus = "locales: [en]\nstatus:\n  gatus: https://status.example.org\n"

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
	m, err := BuildModel(cfg, map[string]bool{"elsewhere": true})
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
