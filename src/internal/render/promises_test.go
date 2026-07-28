package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestAboutHashInPage(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nabout: Hello there\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	home := string(m.Pages["en"].HTML)
	h1 := aboutHash(cfg.Site.About)
	if !strings.Contains(home, `data-about="`+h1+`"`) {
		t.Errorf("home missing data-about=%q", h1)
	}
	if h2 := aboutHash(config.LString{"": "Hello there, edited"}); h2 == h1 {
		t.Error("hash should change when the note changes")
	}
	if aboutHash(nil) != "" {
		t.Error("empty note should have no hash")
	}
}

func TestStarterModelAndInit(t *testing.T) {
	m := StarterModel()
	home := string(m.Pages["en"].HTML)
	for _, want := range []string{"services.yaml", "getting-started", "<code>cairn -init</code>"} {
		if !strings.Contains(home, want) {
			t.Errorf("starter page missing %q", want)
		}
	}
	// What -init prints must itself boot: load it the way a real directory is.
	dir := testutil.WriteFiles(t, map[string]string{"services.yaml": config.StarterServices})
	if _, err := config.Load(dir); err != nil {
		t.Errorf("what cairn -init prints does not load: %v", err)
	}
}
