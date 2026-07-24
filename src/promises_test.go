package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAboutHashInPage(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nabout: Hello there\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	home := string(m.Pages["en"].HTML)
	h1 := aboutHash(cfg.Site.About)
	if !strings.Contains(home, `data-about="`+h1+`"`) {
		t.Errorf("home missing data-about=%q", h1)
	}
	if h2 := aboutHash(LString{"": "Hello there, edited"}); h2 == h1 {
		t.Error("hash should change when the note changes")
	}
	if aboutHash(nil) != "" {
		t.Error("empty note should have no hash")
	}
}

func TestStarterModelAndInit(t *testing.T) {
	m := starterModel()
	home := string(m.Pages["en"].HTML)
	for _, want := range []string{"services.yaml", "getting-started", "<code>cairn -init</code>"} {
		if !strings.Contains(home, want) {
			t.Errorf("starter page missing %q", want)
		}
	}
	// The starter config must itself be a valid services file.
	if _, err := parseServices("starter", []byte(starterServices)); err != nil {
		t.Errorf("starterServices does not parse: %v", err)
	}
}

func TestLocalIconResolution(t *testing.T) {
	assets := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assets, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "icons", "immich.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := assetsPath
	assetsPath = assets
	defer func() { assetsPath = old }()

	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, icon: immich}\n" +
			"- {id: b, url: https://b.example.org, name: B, icon: nextcloud}\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := iconURL(cfg, "immich"); got != "/assets/icons/immich.svg" {
		t.Errorf("local icon = %q, want /assets/icons/immich.svg", got)
	}
	if got := iconURL(cfg, "nextcloud"); !strings.HasPrefix(got, iconCDN) {
		t.Errorf("missing local file should fall back to CDN, got %q", got)
	}
	script := string(emitIconsScript(cfg))
	if strings.Contains(script, "immich") || !strings.Contains(script, "icons/nextcloud.svg") {
		t.Errorf("emit-icons should list only CDN-bound slugs:\n%s", script)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(warnings, "nextcloud") || strings.Contains(warnings, "immich") {
		t.Errorf("check should warn only about CDN slugs:\n%s", warnings)
	}
}
