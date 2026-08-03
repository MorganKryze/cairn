package config_test

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// Both keys are off unless the operator says otherwise, which is what makes
// this addition invisible to every site already running.
func TestServiceLinksDefaultOff(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "title: Test\nlocales: [en]\n",
		"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.ServiceLinks.NewTab {
		t.Error("new_tab defaults on, want off")
	}
	if cfg.Site.ServiceLinks.Confirm != config.ConfirmOff {
		t.Errorf("confirm defaults to %q, want off", cfg.Site.ServiceLinks.Confirm)
	}
}

func TestConfirmScopeSpellings(t *testing.T) {
	for _, tc := range []struct {
		written string
		want    config.ConfirmScope
	}{
		{"all", config.ConfirmAll},
		{"true", config.ConfirmAll}, // a reader who has just written new_tab: true
		{"external", config.ConfirmExternal},
		{"false", config.ConfirmOff},
	} {
		dir := testutil.WriteFiles(t, map[string]string{
			"site.yaml": "title: Test\nlocales: [en]\n" +
				"service_links:\n  new_tab: true\n  confirm: " + tc.written + "\n",
			"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n",
		})
		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("confirm: %s did not load: %v", tc.written, err)
		}
		if got := cfg.Site.ServiceLinks.Confirm; got != tc.want {
			t.Errorf("confirm: %s read back as %q, want %q", tc.written, got, tc.want)
		}
		if !cfg.Site.ServiceLinks.NewTab {
			t.Errorf("confirm: %s lost new_tab", tc.written)
		}
	}
}

// A misspelling names the line and says what the two answers are, rather than
// leaving a dialog silently off on a site that asked for one.
func TestConfirmScopeRefusesAnythingElse(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "title: Test\nlocales: [en]\n" +
			"service_links:\n  confirm: externals\n",
		"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n",
	})
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("confirm: externals loaded, want a refusal")
	}
	for _, want := range []string{"site.yaml", `"externals"`, "external"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%v", want, err)
		}
	}
}

func TestConfirmScopeWants(t *testing.T) {
	for _, tc := range []struct {
		scope config.ConfirmScope
		kind  string
		want  bool
		why   string
	}{
		{config.ConfirmAll, "self", true, "all means all"},
		{config.ConfirmAll, "", true, "all reaches an unflagged service"},
		{config.ConfirmExternal, "external", true, "the flagged case"},
		{config.ConfirmExternal, "self", false, "the host runs this one"},
		{config.ConfirmExternal, "", false, "silence is not a claim that it is external"},
		{config.ConfirmOff, "external", false, "off is off"},
	} {
		if got := tc.scope.Wants(tc.kind); got != tc.want {
			t.Errorf("%q.Wants(%q) = %v, want %v (%s)", tc.scope, tc.kind, got, tc.want, tc.why)
		}
	}
}

// The block is `service_links`, not `links`, and this is the reason: `links`
// has been the header link list since before any of this, and a sequence and a
// mapping cannot share a key. A site that says `links:` must go on getting its
// header links, which is what this pins.
func TestLinksStillMeansTheHeaderLinks(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "title: Test\nlocales: [en]\n" +
			"links:\n  - label: Blog\n    url: https://blog.example.org\n",
		"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Site.Links) != 1 || cfg.Site.Links[0].URL != "https://blog.example.org" {
		t.Fatalf("header links = %+v, want the one blog link", cfg.Site.Links)
	}
	if got := cfg.Site.Links[0].Label.Get("en", "en"); got != "Blog" {
		t.Errorf("header link label = %q, want Blog", got)
	}
}
