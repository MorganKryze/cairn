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
