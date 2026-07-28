package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestManyLocales(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [fr, en, de, es, it]\ntitle: { fr: Chez nous, en: Our place }\n",
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
	de := string(m.Pages["de"].HTML)
	if !strings.Contains(de, `<details class="langs langs-menu">`) {
		t.Error("five locales should fold the switcher into a menu")
	}
	if !strings.Contains(de, "Werkzeug suchen…") {
		t.Error("german page should use the german built-in strings")
	}
	fr := string(m.Pages["fr"].HTML)
	if !strings.Contains(fr, "Chez nous") || !strings.Contains(string(m.Pages["en"].HTML), "Our place") {
		t.Error("title should localize per page")
	}
}

func TestFewLocalesKeepInlineSwitcher(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [fr, en]\n",
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
	home := string(m.Pages["fr"].HTML)
	if strings.Contains(home, "langs-menu") || !strings.Contains(home, `<nav class="langs"`) {
		t.Error("two locales should keep the inline switcher")
	}
}
