package render

import (
	"regexp"
	"strings"
	"testing"
)

func styleOf(t *testing.T, html string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindStringSubmatch(html)
	if m == nil {
		t.Fatal("inline style not found in the rendered page")
	}
	return m[1]
}

// The markup keeps the light image, and the dark one arrives as a rule. That
// split is the whole design: a second <img> would be fetched even hidden, and
// a <picture> follows the operating system rather than the theme button, so
// a visitor who switches by hand would keep the wrong artwork.
func TestAThemedLogoShipsTheLightOneAndARuleForTheDark(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [en]\nlogo:\n  light: /assets/logo.svg\n  dark: /assets/logo-white.svg\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html := string(m.Pages["en"].HTML)
	if !strings.Contains(html, `<img src="/assets/logo.svg" alt="">`) {
		t.Error("the header does not carry the light logo")
	}
	if strings.Contains(html, "/assets/logo-white.svg\" alt") {
		t.Error("the dark logo is in the markup, so both would be fetched")
	}
	style := styleOf(t, html)
	for _, want := range []string{
		`:root[data-theme="dark"] .brand img{content:url("/assets/logo-white.svg")}`,
		`@media(prefers-color-scheme:dark){`,
	} {
		if !strings.Contains(style, want) {
			t.Errorf("inline style missing %s:\n%s", want, style)
		}
	}
	// The style is hashed into the CSP, so a rule that is not counted is a
	// page that renders unstyled.
	if got := cspHash(style); !strings.Contains(m.CSP, got) {
		t.Errorf("the themed style hashes to %s, absent from the CSP", got)
	}
}

// Service icons are the case that repeats, so the rule is keyed on the pair
// rather than on the service: twenty cards sharing an icon carry one rule.
func TestServicesSharingAThemedIconShareOneRule(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml": "locales: [en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, icon: {light: github, dark: github-light}}\n" +
			"- {id: b, url: https://b.example.org, name: B, icon: {light: github, dark: github-light}}\n" +
			"- {id: c, url: https://c.example.org, name: C, icon: {light: gitlab, dark: gitlab-light}}\n",
	})
	html := string(m.Pages["en"].HTML)
	style := styleOf(t, html)
	if n := strings.Count(style, "github-light.svg"); n != 2 {
		t.Errorf("github-light appears %d times in the style, want 2 (one per theme selector, not one per card):\n%s", n, style)
	}
	if !strings.Contains(style, "gitlab-light.svg") {
		t.Errorf("the second pair earned no rule:\n%s", style)
	}
	// Whatever class carries the rule has to be on both cards that asked for it.
	class := regexp.MustCompile(`\.([a-z0-9-]+)\{content:url\("[^"]*github-light`).FindStringSubmatch(style)
	if class == nil {
		t.Fatalf("no class carries the github rule:\n%s", style)
	}
	if n := strings.Count(html, `class="tile"`); n != 3 {
		t.Errorf("expected three tiles, found %d", n)
	}
	if n := strings.Count(html, class[1]); n < 3 {
		t.Errorf("class %q appears %d times in the page, want it on both cards and in the rule", class[1], n)
	}
}

// A site that themes nothing renders exactly the style it rendered before, so
// the golden pages and every existing CSP hash stay where they are.
func TestASiteThatThemesNothingCarriesNoExtraRule(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [en]\nlogo: /assets/logo.svg\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, icon: github}\n",
	})
	if style := styleOf(t, string(m.Pages["en"].HTML)); style != ":root{--accent:#247b7b}" {
		t.Errorf("the inline style grew for a site that themes nothing: %q", style)
	}
}

// The favicon cannot be reached by the page's stylesheet, so it is the one
// surface that gets a second <link> and follows the operating system.
func TestAThemedFaviconEmitsAMediaLink(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [en]\nfavicon:\n  light: /assets/fav.svg\n  dark: /assets/fav-white.svg\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	html := string(m.Pages["en"].HTML)
	if !strings.Contains(html, `<link rel="icon" href="/assets/fav-white.svg" media="(prefers-color-scheme: dark)">`) {
		t.Error("no dark favicon link")
	}
	if !strings.Contains(html, `<link rel="icon" href="/assets/fav.svg">`) {
		t.Error("the light favicon link went missing")
	}
	// The dark one has to come first: a browser that ignores media takes the
	// last icon link it understands, and that has to be the light one.
	if strings.Index(html, "fav-white.svg") > strings.Index(html, `href="/assets/fav.svg"`) {
		t.Error("the dark link is declared after the light one, so a browser ignoring media picks the wrong file")
	}
}
