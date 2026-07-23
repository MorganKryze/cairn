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
	if !strings.Contains(home, `href="https://wiki.example.org"`) || !strings.Contains(home, `class="menu-link"`) {
		t.Error("home header missing the nav links")
	}
	if !strings.Contains(home, `href="/fr/legal/"`) || !strings.Contains(home, "Mentions légales") {
		t.Error("footer missing the auto page link")
	}
	legal, ok := m.Pages["fr/legal"]
	if !ok {
		t.Fatal("page fr/legal not rendered")
	}
	if h := string(legal.HTML); !strings.Contains(h, "<h1>Mentions légales</h1>") || strings.Count(h, `<p class="page-intro">Éditeur : Morgan.</p>`) != 1 || !strings.Contains(h, `<p class="page-intro">Hébergeur : lui-même.</p>`) {
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

func TestAboutAndLinkIcons(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "about: { fr: \"Bienvenue chez moi.\\n\\nBonne visite.\", en: Welcome }\n" +
			"links:\n  - { label: Contact, url: mailto:a@b.c, icon: mail }\n" +
			"locales: [fr, en]\n",
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
	if !strings.Contains(home, `class="about"`) || !strings.Contains(home, "<p>Bienvenue chez moi.</p>") || !strings.Contains(home, "<p>Bonne visite.</p>") {
		t.Error("home missing the about block or its paragraphs")
	}
	if !strings.Contains(home, ">Masquer</span>") {
		t.Error("dismiss button should carry the localized label")
	}
	if !strings.Contains(home, `class="menu-link"`) || !strings.Contains(home, "<svg") {
		t.Error("menu link should render its built-in glyph")
	}
	if !strings.Contains(home, `id="search"`) {
		t.Error("search should render in the header menu row")
	}

	dir = writeFiles(t, map[string]string{
		"site.yaml":     "links: [{label: X, url: https://x.example.org, icon: sparkles}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "built-in glyph") {
		t.Errorf("error = %v, want unknown glyph complaint", err)
	}
}

func TestUnknownServiceKeyIsAnError(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, descr: typo}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "descr") {
		t.Errorf("error = %v, want a complaint naming the unknown key descr", err)
	}
	dir = writeFiles(t, map[string]string{
		"categories.yaml": "- {id: docs, nam: Documents}\n",
		"services.yaml":   "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "nam") {
		t.Errorf("error = %v, want a complaint naming the unknown key nam", err)
	}
}

func TestPageSections(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "pages:\n  - id: legal\n    title: Legal\n    sections:\n" +
			"      - { title: Publisher, body: \"First Last.\" }\n" +
			"      - { title: Hosting, body: \"Self-hosted.\" }\n",
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
	h := string(m.Pages["en/legal"].HTML)
	if !strings.Contains(h, "<h2>Publisher</h2>") || !strings.Contains(h, "<h2>Hosting</h2>") || !strings.Contains(h, "<p>Self-hosted.</p>") {
		t.Error("sections should render as titled blocks")
	}
	if !strings.Contains(h, `content="First Last."`) {
		t.Error("first section paragraph should feed the meta description")
	}

	dir = writeFiles(t, map[string]string{
		"site.yaml":     "pages: [{id: legal, title: T}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "body or sections") {
		t.Errorf("error = %v, want body-or-sections complaint", err)
	}
}
