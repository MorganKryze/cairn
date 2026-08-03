package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestLoadConfigMergesAndGroups(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":  "title: Test\nlocales: [fr, en]\n",
		"docs.yaml":  "- id: pdf\n  url: https://pdf.example.org\n  category: documents\n  name: {fr: PDF, en: PDF}\n",
		"media.yaml": "- id: tube\n  url: https://tube.example.org\n  name: Tube\n",
	})
	cfg, err := config.Load(dir)
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
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: A\n- id: b\n  name: B\n",
	})
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "services.yaml line 4") || !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("error = %v, want file, line 4 and service id", err)
	}
}

func TestLoadConfigRejectsDuplicateIDs(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"a.yaml": "- {id: x, url: https://x.example.org, name: X}\n",
		"b.yaml": "- {id: x, url: https://x.example.org, name: X}\n",
	})
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "a.yaml") || !strings.Contains(err.Error(), "b.yaml") {
		t.Errorf("error = %v, want both file names", err)
	}
}

func TestCategoriesMetaOrderAndNames(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, category: alpha}\n" +
			"- {id: b, url: https://b.example.org, name: B, category: beta}\n" +
			"- {id: c, url: https://c.example.org, name: C, category: gamma}\n" +
			"- {id: d, url: https://d.example.org, name: D}\n",
		"categories.yaml": "- id: gamma\n  name: {fr: Trucs, en: Stuff}\n  order: 1\n- id: beta\n  order: 2\n- id: unused\n  order: 3\n",
	})
	cfg, err := config.Load(dir)
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
	if got := cfg.CategoryName(cfg.Categories[0], "fr"); got != "Trucs" {
		t.Errorf("localized name = %q, want Trucs", got)
	}
	if got := cfg.CategoryName(cfg.Categories[2], "fr"); got != "Alpha" {
		t.Errorf("derived name = %q, want Alpha", got)
	}
}

func TestCategoriesErrorsNameFileAndLine(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml":   "- {id: a, url: https://a.example.org, name: A}\n",
		"categories.yaml": "- id: ok\n- name: {fr: Sans id}\n",
	})
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "categories.yaml line 2") || !strings.Contains(err.Error(), "missing id") {
		t.Errorf("error = %v, want categories.yaml line 2 missing id", err)
	}
}

func TestDetailPages(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n",
		"services.yaml": "- id: pdf\n  url: https://pdf.example.org\n  name: PDF\n" +
			"  details: {fr: \"Un.\\n\\nDeux.\", en: One.}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, nil)
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
	if !strings.Contains(html, "Ouvrir") {
		t.Error("detail page missing localized open button")
	}
	if !strings.Contains(string(m.Pages["fr"].HTML), `href="/fr/pdf/"`) {
		t.Error("home card missing more-link to detail page")
	}
}

func TestServiceImages(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
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
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, nil)
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
		if _, err := config.Load(testutil.WriteFiles(t, files)); err == nil {
			t.Errorf("%s: want an error, got none", name)
		}
	}
}

// A file in media/ has two documented spellings, the bare name and the
// /media/… path it is served at, and they name the same file on disk. Only the
// bare one was ever stat'd: the same missing image refused to boot written one
// way and reached the page as a broken img written the other, with -check
// printing ok both times.
func TestAbsoluteMediaPathIsCheckedLikeABareName(t *testing.T) {
	for _, src := range []string{"nope.png", "/media/nope.png"} {
		dir := testutil.WriteFiles(t, map[string]string{
			"services.yaml": "- {id: a, url: https://a.example.org, name: A, images: [" + src + "]}\n",
		})
		_, err := config.Load(dir)
		if err == nil {
			t.Errorf("image %q: a file that is not there was accepted", src)
			continue
		}
		// The message quotes the spelling the operator wrote, not the one the
		// check normalised to, or they cannot find the line to fix.
		for _, want := range []string{`service "a"`, `"` + src + `"`, "not found"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("image %q: error %v does not mention %q", src, err, want)
			}
		}
	}
}

// The same path with the file in place still loads. The rule is about the
// file, not about the slash.
func TestAbsoluteMediaPathAcceptsAFileThatIsThere(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, images: [/media/shot.png]}\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "shot.png"), []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(dir); err != nil {
		t.Fatalf("refused an image that is on disk: %v", err)
	}
}

// Everything cairn does not serve itself keeps passing through unlooked at,
// with no media/ directory here at all. None of these names a file this
// directory is responsible for, and stat'ing them would refuse configs that
// are correct.
func TestImagesCairnDoesNotServeAreNotCheckedForExistence(t *testing.T) {
	for _, src := range []string{
		"https://cdn.example.org/shot.png", // somebody else's server
		"//cdn.example.org/shot.png",       // the protocol-relative form of it
		"/assets/shot.png",                 // the mounted assets dir, not this one
		"/static/shot.png",                 // cairn's own bundled files
		"/whatever/shot.png",               // anything else behind the base path
	} {
		dir := testutil.WriteFiles(t, map[string]string{
			"services.yaml": "- {id: a, url: https://a.example.org, name: A, images: [\"" + src + "\"]}\n",
		})
		if _, err := config.Load(dir); err != nil {
			t.Errorf("image %q was checked for existence and should not have been: %v", src, err)
		}
	}
}

func TestFriendlyYAMLErrors(t *testing.T) {
	for _, tc := range []struct{ name, services, want string }{
		{"scalar for list", "- id: a\n  url: https://a.example.org\n  name: A\n  tags: solo\n",
			"found a text `solo` where a list of texts was expected"},
		{"list for text", "- id: a\n  url: https://a.example.org\n  name: [Un, Deux]\n",
			"one plain text or a per-locale mapping"},
		{"comma in flow map", "- id: a\n  url: https://a.example.org\n  name: A\n  desc: { fr: Une phrase, avec virgule., en: Fine. }\n",
			"is not a locale code"},
	} {
		_, err := config.Load(testutil.WriteFiles(t, map[string]string{"services.yaml": tc.services}))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
	svc := "- {id: a, url: https://a.example.org, name: A}\n"
	_, err := config.Load(testutil.WriteFiles(t, map[string]string{"site.yaml": "locales: fr\n", "services.yaml": svc}))
	if err == nil || !strings.Contains(err.Error(), "where a list of texts was expected") {
		t.Errorf("site error = %v, want the humanized list message", err)
	}
	_, err = config.Load(testutil.WriteFiles(t, map[string]string{"site.yaml": "titel: Oops\n", "services.yaml": svc}))
	if err == nil || !strings.Contains(err.Error(), `unknown key "titel"`) || strings.Contains(err.Error(), "main.") {
		t.Errorf("site error = %v, want unknown key without Go type names", err)
	}
}

func TestServiceIDValidation(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: Bad ID, url: https://a.example.org, name: A}\n",
	})
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "invalid id") {
		t.Errorf("error = %v, want invalid id", err)
	}
}

func TestCustomCSSDetection(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
		"custom.css":    "body { background: pink }\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CustomCSS {
		t.Error("CustomCSS = false, want true")
	}
}

func TestStrOverrideAndFallback(t *testing.T) {
	cfg := &config.Config{Site: config.Site{
		Locales: []string{"fr", "en"},
		Strings: map[string]config.LString{"search.placeholder": {"fr": "Chercher…"}},
	}}
	if got := cfg.Str("fr", "search.placeholder"); got != "Chercher…" {
		t.Errorf("override = %q", got)
	}
	if got := cfg.Str("en", "search.placeholder"); got != "Search for a tool…" {
		t.Errorf("builtin fallback = %q", got)
	}
	if got := cfg.Str("xx", "search.empty"); got != "No results. Try another word." {
		t.Errorf("english fallback = %q", got)
	}
	if got := cfg.Str("pt-BR", "card.more"); got != "Saber mais" {
		t.Errorf("base-locale fallback = %q, want the pt table", got)
	}
}
