package render

import (
	"regexp"
	"strings"
	"testing"
)

// The CSP only works if its hashes match the inline fragments the template
// actually renders. Extract them from a real page and check, so a stray edit
// to layout.tmpl or prePaintScript breaks the build instead of the site.
func TestCSPHashesMatchTheRenderedPage(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [fr]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	csp := m.CSP
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("csp = %q, missing base directives", csp)
	}

	html := string(m.Pages["fr"].HTML)
	script := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindStringSubmatch(html)
	style := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindStringSubmatch(html)
	if script == nil || style == nil {
		t.Fatal("inline script or style not found in the rendered page")
	}
	if got := cspHash(script[1]); !strings.Contains(csp, got) {
		t.Errorf("rendered inline script hashes to %s, absent from the CSP: layout.tmpl and prePaintScript diverged", got)
	}
	if got := cspHash(style[1]); !strings.Contains(csp, got) {
		t.Errorf("rendered accent style hashes to %s, absent from the CSP", got)
	}
}

// A site that sets theme.font makes the inline style carry an @font-face and
// a --font-body override, both of which change the style's hash. The CSP has
// to allow the new block or every page of that site renders unstyled, so the
// same check runs against a themed config.
func TestCSPHashesMatchAFontThemedPage(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [fr]\ntheme:\n  font:\n    family: '\"Inter\", system-ui, sans-serif'\n    file: fonts/custom-font.woff2\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html := string(m.Pages["fr"].HTML)
	style := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindStringSubmatch(html)
	if style == nil {
		t.Fatal("inline style not found in the rendered page")
	}
	for _, want := range []string{
		`@font-face{font-family:"Inter";src:url("/fonts/custom-font.woff2") format("woff2")`,
		`--font-body:"Inter", system-ui, sans-serif`,
	} {
		if !strings.Contains(style[1], want) {
			t.Errorf("inline style missing %s:\n%s", want, style[1])
		}
	}
	if got := cspHash(style[1]); !strings.Contains(m.CSP, got) {
		t.Errorf("rendered themed style hashes to %s, absent from the CSP", got)
	}
	if !strings.Contains(m.CSP, "font-src 'self'") {
		t.Errorf("csp %q has no font-src 'self', so the self-hosted font would be blocked", m.CSP)
	}
}
