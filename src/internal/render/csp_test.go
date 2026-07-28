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
