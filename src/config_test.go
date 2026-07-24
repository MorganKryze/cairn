package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadConfigMergesAndGroups(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml":  "title: Test\nlocales: [fr, en]\n",
		"docs.yaml":  "- id: pdf\n  url: https://pdf.example.org\n  category: documents\n  name: {fr: PDF, en: PDF}\n",
		"media.yaml": "- id: tube\n  url: https://tube.example.org\n  name: Tube\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultLocale() != "fr" {
		t.Errorf("default locale = %q, want fr", cfg.DefaultLocale())
	}
	if len(cfg.Categories) != 2 || cfg.Categories[0].ID != "documents" || cfg.Categories[1].ID != "other" {
		t.Errorf("categories = %+v, want [documents other]", cfg.Categories)
	}
	if got := cfg.Categories[1].Services[0].Name.Get("fr", "en"); got != "Tube" {
		t.Errorf("plain-string name = %q, want Tube", got)
	}
}

func TestLoadConfigErrorsNameFileAndLine(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: A\n- id: b\n  name: B\n",
	})
	_, err := loadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "services.yaml line 4") || !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("error = %v, want file, line 4 and service id", err)
	}
}

func TestLoadConfigRejectsDuplicateIDs(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.yaml": "- {id: x, url: https://x.example.org, name: X}\n",
		"b.yaml": "- {id: x, url: https://x.example.org, name: X}\n",
	})
	_, err := loadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "a.yaml") || !strings.Contains(err.Error(), "b.yaml") {
		t.Errorf("error = %v, want both file names", err)
	}
}

func TestCategoriesMetaOrderAndNames(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, category: alpha}\n" +
			"- {id: b, url: https://b.example.org, name: B, category: beta}\n" +
			"- {id: c, url: https://c.example.org, name: C, category: gamma}\n" +
			"- {id: d, url: https://d.example.org, name: D}\n",
		"categories.yaml": "- id: gamma\n  name: {fr: Trucs, en: Stuff}\n  order: 1\n- id: beta\n  order: 2\n- id: unused\n  order: 3\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range cfg.Categories {
		ids = append(ids, c.ID)
	}
	if got, want := strings.Join(ids, " "), "gamma beta alpha other"; got != want {
		t.Errorf("order = %q, want %q", got, want)
	}
	if got := cfg.categoryName(cfg.Categories[0], "fr"); got != "Trucs" {
		t.Errorf("localized name = %q, want Trucs", got)
	}
	if got := cfg.categoryName(cfg.Categories[2], "fr"); got != "Alpha" {
		t.Errorf("derived name = %q, want Alpha", got)
	}
}

func TestCategoriesErrorsNameFileAndLine(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"services.yaml":   "- {id: a, url: https://a.example.org, name: A}\n",
		"categories.yaml": "- id: ok\n- name: {fr: Sans id}\n",
	})
	_, err := loadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "categories.yaml line 2") || !strings.Contains(err.Error(), "missing id") {
		t.Errorf("error = %v, want categories.yaml line 2 missing id", err)
	}
}

func TestDetailPages(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n",
		"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n" +
			"  details: {fr: \"Un.\\n\\nDeux.\", en: One.}\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"fr", "en", "fr/pdf", "en/pdf"} {
		if _, ok := m.Pages[key]; !ok {
			t.Errorf("missing page %q", key)
		}
	}
	html := string(m.Pages["fr/pdf"].HTML)
	if !strings.Contains(html, "<p>Un.</p>") || !strings.Contains(html, "<p>Deux.</p>") {
		t.Errorf("details not split into paragraphs:\n%s", html)
	}
	if !strings.Contains(html, "Ouvrir l’outil") {
		t.Error("detail page missing localized open button")
	}
	if !strings.Contains(string(m.Pages["fr"].HTML), `href="/fr/pdf/"`) {
		t.Error("home card missing more-link to detail page")
	}
}

func TestServiceImages(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n",
		"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n  images:\n" +
			"    - pdf.png\n" +
			"    - src: https://example.org/far.png\n      caption: {fr: Légende, en: Caption}\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "pdf.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	html := string(m.Pages["fr/pdf"].HTML)
	if !strings.Contains(html, `<img src="/media/pdf.png"`) {
		t.Errorf("bare image name not resolved to /media/:\n%s", html)
	}
	if !strings.Contains(html, `src="https://example.org/far.png"`) || !strings.Contains(html, "<figcaption>Légende</figcaption>") {
		t.Errorf("URL image or localized caption missing:\n%s", html)
	}
	if !strings.Contains(string(m.Pages["fr"].HTML), `href="/fr/pdf/"`) {
		t.Error("images alone should enable the card more-link")
	}
}

func TestServiceImageErrors(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"missing file": {
			"services.yaml": "- {id: a, url: https://a.example.org, name: A, images: [nope.png]}\n",
		},
		"traversal": {
			"services.yaml": "- {id: a, url: https://a.example.org, name: A, images: [../site.yaml]}\n",
		},
		"unknown key": {
			"services.yaml": "- id: a\n  url: https://a.example.org\n  name: A\n  images:\n    - {src: a.png, captoin: typo}\n",
		},
	} {
		if _, err := loadConfig(writeFiles(t, files)); err == nil {
			t.Errorf("%s: want an error, got none", name)
		}
	}
}

func TestServiceIDValidation(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: Bad ID, url: https://a.example.org, name: A}\n",
	})
	_, err := loadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "invalid id") {
		t.Errorf("error = %v, want invalid id", err)
	}
}

func TestCustomCSSDetection(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
		"custom.css":    "body { background: pink }\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CustomCSS {
		t.Error("CustomCSS = false, want true")
	}
}

func TestStrOverrideAndFallback(t *testing.T) {
	cfg := &Config{Site: Site{
		Locales: []string{"fr", "en"},
		Strings: map[string]LString{"search.placeholder": {"fr": "Chercher…"}},
	}}
	if got := cfg.Str("fr", "search.placeholder"); got != "Chercher…" {
		t.Errorf("override = %q", got)
	}
	if got := cfg.Str("en", "search.placeholder"); got != "Search for a tool…" {
		t.Errorf("builtin fallback = %q", got)
	}
	if got := cfg.Str("de", "search.empty"); got != "No results. Try another word." {
		t.Errorf("english fallback = %q", got)
	}
}

func TestNegotiate(t *testing.T) {
	locales := []string{"fr", "en"}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Language", "de;q=0.9, en-GB;q=0.8")
	if got := negotiate(r, locales); got != "en" {
		t.Errorf("header negotiation = %q, want en", got)
	}
	r.AddCookie(&http.Cookie{Name: "locale", Value: "fr"})
	if got := negotiate(r, locales); got != "fr" {
		t.Errorf("cookie should win, got %q", got)
	}
	bare := httptest.NewRequest("GET", "/", nil)
	if got := negotiate(bare, locales); got != "fr" {
		t.Errorf("default = %q, want fr", got)
	}
}
