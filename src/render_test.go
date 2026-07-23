package main

import (
	"strings"
	"testing"
)

func TestCategoryTrail(t *testing.T) {
	m, err := buildModel(testConfig(t), nil) // two categories: documents + other
	if err != nil {
		t.Fatal(err)
	}
	html := string(m.Pages["fr"].HTML)
	if !strings.Contains(html, `class="toc"`) || !strings.Contains(html, `href="#cat-documents"`) {
		t.Error("two categories should render the trail")
	}
	if !strings.Contains(html, `aria-label="Catégories"`) {
		t.Error("trail label should be localized")
	}

	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, category: docs}\n" +
			"- {id: b, url: https://b.example.org, name: B, category: docs}\n",
	})
	solo, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm, err := buildModel(solo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sm.Pages["en"].HTML), `class="toc"`) {
		t.Error("a single category needs no trail")
	}
}
