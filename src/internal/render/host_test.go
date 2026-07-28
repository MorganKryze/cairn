package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestHostFlag(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en, fr]\n",
		"services.yaml": "- {id: mine, url: https://mine.example.org, name: Mine, selfhosted: true}\n" +
			"- {id: theirs, url: https://theirs.example.org, name: Theirs, selfhosted: false}\n" +
			"- {id: plain, url: https://plain.example.org, name: Plain}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	en := string(m.Pages["en"].HTML)
	if !strings.Contains(en, `class="flag flag-self"`) || !strings.Contains(en, "Self-hosted") {
		t.Error("selfhosted: true should render the self flag with its label")
	}
	if !strings.Contains(en, `class="flag flag-external"`) || !strings.Contains(en, "External") {
		t.Error("selfhosted: false should render the external flag with its label")
	}
	// Exactly two flags: the third service sets nothing and carries none.
	if n := strings.Count(en, `class="flag `); n != 2 {
		t.Errorf("got %d flags, want 2 (the plain service has none)", n)
	}

	// Labels follow the locale.
	fr := string(m.Pages["fr"].HTML)
	if !strings.Contains(fr, "Auto-hébergé") || !strings.Contains(fr, "Externe") {
		t.Error("flag labels should be localized in French")
	}
}
