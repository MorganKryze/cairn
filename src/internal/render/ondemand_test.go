package render

import (
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
