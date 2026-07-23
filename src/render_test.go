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

func TestSitePagesAndQuickLinks(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n" +
			"links:\n  - { label: Wiki, url: https://wiki.example.org }\n" +
			"pages:\n  - id: legal\n    title: { fr: Mentions légales, en: Legal notice }\n" +
			"    body: { fr: \"Éditeur : Morgan.\\n\\nHébergeur : lui-même.\", en: \"Publisher: Morgan.\" }\n",
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
	home := string(m.Pages["fr"].HTML)
	if !strings.Contains(home, `href="https://wiki.example.org"`) || !strings.Contains(home, `class="quick"`) {
		t.Error("home header missing the quick links")
	}
	if !strings.Contains(home, `href="/fr/legal/"`) || !strings.Contains(home, "Mentions légales") {
		t.Error("footer missing the auto page link")
	}
	legal, ok := m.Pages["fr/legal"]
	if !ok {
		t.Fatal("page fr/legal not rendered")
	}
	if h := string(legal.HTML); !strings.Contains(h, "<h1>Mentions légales</h1>") || strings.Count(h, "<p>Éditeur : Morgan.</p>") != 1 || !strings.Contains(h, "<p>Hébergeur : lui-même.</p>") {
		t.Error("page body should render title and blank-line paragraphs")
	}

	dir = writeFiles(t, map[string]string{
		"site.yaml":     "pages: [{id: a, title: T, body: B}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Errorf("error = %v, want page/service id collision complaint", err)
	}
}
