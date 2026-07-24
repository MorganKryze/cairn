package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckWarnings(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nabout: {fr: Bonjour}\n",
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: {fr: Outil, en: Tool}\n  desc: {fr: Une phrase}\n" +
			"- id: b\n  url: https://b.example.org\n  name: Partout\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "orphan.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{
		"site about has no en",
		`service "a" desc has no en`,
		"media/orphan.png is not referenced",
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	if strings.Contains(warnings, `"b"`) {
		t.Errorf("plain-string service should not warn:\n%s", warnings)
	}
}

func TestCheckMarkdownImageCountsAsUsed(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nabout: \"See ![plan](plan.png)\"\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "plan.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := checkWarnings(cfg, dir); len(w) != 0 {
		t.Errorf("warnings = %v, want none", w)
	}
}
