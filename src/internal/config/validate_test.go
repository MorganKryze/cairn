package config

import (
	"strings"
	"testing"
)

// Every rejection here is a message an operator reads at 2am with a site that
// will not boot. The point of the table is not the coverage number: it is that
// each refusal keeps naming the field and showing the expected shape, which is
// what the README promises.
func TestValidateSiteRejections(t *testing.T) {
	base := func() *Site {
		s := &Site{Locales: []string{"en"}}
		s.Theme.Accent = "#247b7b"
		return s
	}

	for _, c := range []struct {
		name  string
		mut   func(*Site)
		wants []string // fragments the message must carry
	}{
		// the POSIX habit: fr_FR rather than fr-FR. The rule is otherwise
		// permissive on purpose, an unknown-but-well-formed tag just falls
		// back to English.
		{"locale written the POSIX way", func(s *Site) { s.Locales = []string{"fr_FR"} },
			[]string{"invalid locale", `"fr_FR"`, "pt-BR"}},
		{"locale starting with a digit", func(s *Site) { s.Locales = []string{"1fr"} },
			[]string{"invalid locale"}},
		{"accent that is not hex", func(s *Site) { s.Theme.Accent = "teal" },
			[]string{"theme.accent", "hex color", "#247b7b"}},
		{"gatus that is not a URL", func(s *Site) { s.Status.Gatus = "status.example.org" },
			[]string{"status.gatus", "is not a URL"}},
		{"status page that is not a URL", func(s *Site) { s.Status.Page = "nope" },
			[]string{"status.page", "is not a URL"}},
		{"interval that is not a duration", func(s *Site) { s.Status.Interval = "often" },
			[]string{"status.interval", "at least 5s"}},
		{"interval below the floor", func(s *Site) { s.Status.Interval = "1s" },
			[]string{"status.interval", "at least 5s"}},
		{"site url that is not a URL", func(s *Site) { s.URL = "tools.example.org" },
			[]string{"url", "is not a URL"}},
		// The mistake this field invites: writing the address rather than the
		// URI. RFC 9116 wants a scheme, and a bare address makes the file
		// unparseable for the researcher it was written for.
		{"security contact without a scheme", func(s *Site) { s.Security.Contact = "security@example.org" },
			[]string{"security.contact", "is not a URI", "mailto:"}},
		{"security policy without a scheme", func(s *Site) {
			s.Security.Contact = "mailto:security@example.org"
			s.Security.Policy = "example.org/policy"
		}, []string{"security.policy", "is not a URI"}},
		// security.txt is line-based, so a newline in a value forges a field.
		{"security contact carrying a newline", func(s *Site) {
			s.Security.Contact = "mailto:a@example.org\nExpires: 1999-01-01T00:00:00Z"
		}, []string{"security.contact", "one line"}},
		{"security without a contact", func(s *Site) { s.Security.Policy = "https://example.org/policy" },
			[]string{"security needs a contact"}},
		{"link without a label", func(s *Site) { s.Links = []FooterLink{{URL: "https://w.example.org"}} },
			[]string{"links entry needs label and url"}},
		{"link without a url", func(s *Site) { s.Links = []FooterLink{{Label: LString{"": "Wiki"}}} },
			[]string{"links entry needs label and url"}},
		{"link icon that is not a glyph", func(s *Site) {
			s.Links = []FooterLink{{Label: LString{"": "Wiki"}, URL: "https://w.example.org", Icon: "sparkle"}}
		}, []string{"is not a built-in glyph", "sparkle"}},
		{"page id that is not url-safe", func(s *Site) {
			s.Pages = []SitePage{{ID: "Legal Notice", Title: LString{"": "x"}, Body: LString{"": "y"}}}
		}, []string{"invalid page id", "ids become URLs"}},
		{"page without a body or sections", func(s *Site) {
			s.Pages = []SitePage{{ID: "legal", Title: LString{"": "x"}}}
		}, []string{"needs a title and a body or sections"}},
		{"page without a title", func(s *Site) {
			s.Pages = []SitePage{{ID: "legal", Body: LString{"": "y"}}}
		}, []string{"needs a title and a body or sections"}},
		{"duplicate page id", func(s *Site) {
			p := SitePage{ID: "legal", Title: LString{"": "x"}, Body: LString{"": "y"}}
			s.Pages = []SitePage{p, p}
		}, []string{"duplicate page id", "legal"}},
		// The icons list is the one place an operator states a size cairn will
		// republish verbatim in the manifest, so a typo has to stop at load
		// rather than reach a phone as a malformed entry.
		{"icon without a size", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "/assets/i.png"}}
		}, []string{"icons entry 1", "needs src and sizes"}},
		{"icon without a src", func(s *Site) {
			s.Icons = []SiteIcon{{Sizes: "512x512"}}
		}, []string{"icons entry 1", "needs src and sizes"}},
		{"icon src that is neither URL nor path", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "brand.png", Sizes: "512x512"}}
		}, []string{"icons entry 1", "not a URL or an /assets path"}},
		{"size written the wrong way", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "/assets/i.png", Sizes: "512"}}
		}, []string{"icons entry 1", "is not a size", "512x512"}},
		{"size in a unit", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "/assets/i.png", Sizes: "512px"}}
		}, []string{"is not a size"}},
		{"an unknown purpose", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "/assets/i.png", Sizes: "512x512", Purpose: "badge"}}
		}, []string{"icons entry 1", "badge", "any, maskable, monochrome"}},
		{"a good purpose next to a bad one", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "/assets/i.png", Sizes: "512x512", Purpose: "any sparkly"}}
		}, []string{"sparkly"}},
		// The index in the message is the entry's, not the slice's, so the
		// operator counts entries the way they wrote them.
		{"the second entry is named second", func(s *Site) {
			s.Icons = []SiteIcon{
				{Src: "/assets/a.png", Sizes: "192x192"},
				{Src: "/assets/b.png", Sizes: "nope"},
			}
		}, []string{"icons entry 2"}},
		{"section without a body", func(s *Site) {
			s.Pages = []SitePage{{ID: "legal", Title: LString{"": "x"},
				Sections: []PageSection{{Title: LString{"": "Publisher"}}}}}
		}, []string{"every section needs title and body"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := base()
			c.mut(s)
			err := validateSite(s, map[string]string{})
			if err == nil {
				t.Fatal("accepted a config that should have been refused")
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q does not mention %q", err, want)
				}
			}
		})
	}
}

// A page cannot borrow a service's id: they share the URL space, and the
// message has to say which file the service came from or the operator hunts.
func TestValidateSiteRejectsPageServiceCollision(t *testing.T) {
	s := &Site{Locales: []string{"en"}, Pages: []SitePage{
		{ID: "pad", Title: LString{"": "Pad"}, Body: LString{"": "text"}},
	}}
	s.Theme.Accent = "#247b7b"
	err := validateSite(s, map[string]string{"pad": "services.yaml"})
	if err == nil {
		t.Fatal("a page id colliding with a service id was accepted")
	}
	for _, want := range []string{"collides", "pad", "services.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err, want)
		}
	}
}

// The normalizing half: no locales means English, and a trailing slash on the
// public URL is trimmed so canonical links do not end up doubled.
func TestValidateSiteNormalizes(t *testing.T) {
	s := &Site{URL: "https://tools.example.org/"}
	s.Theme.Accent = "#247b7b"
	if err := validateSite(s, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if len(s.Locales) != 1 || s.Locales[0] != "en" {
		t.Errorf("locales = %v, want [en] by default", s.Locales)
	}
	if s.URL != "https://tools.example.org" {
		t.Errorf("url = %q, want the trailing slash trimmed", s.URL)
	}
}

// The shapes that must pass, so a tightened rule cannot quietly start
// refusing a config that has always been legal.
func TestValidateSiteAccepts(t *testing.T) {
	s := &Site{
		Locales: []string{"fr", "en", "pt-BR"},
		URL:     "http://tools.example.org",
		Links: []FooterLink{
			{Label: LString{"": "Wiki"}, URL: "https://w.example.org", Icon: "book"},
			{Label: LString{"": "Logo"}, URL: "https://x.example.org", Icon: "/assets/l.png"},
			{Label: LString{"": "Plain"}, URL: "https://y.example.org"},
		},
		Pages: []SitePage{
			{ID: "legal-notice", Title: LString{"": "Legal"}, Body: LString{"": "text"}},
			{ID: "privacy", Title: LString{"": "Privacy"},
				Sections: []PageSection{{Title: LString{"": "Data"}, Body: LString{"": "None"}}}},
		},
		Icons: []SiteIcon{
			{Src: "/assets/i-192.png", Sizes: "192x192"},
			{Src: "/assets/i-512.png", Sizes: "512x512", Purpose: "any maskable"},
			{Src: "https://cdn.example.org/i.svg", Sizes: "any"},
			{Src: "/assets/i.png", Sizes: "48x48 96x96"}, // an ico carries several
			{Src: "/assets/i-mono.svg", Sizes: "any", Purpose: "monochrome"},
		},
	}
	s.Theme.Accent = "#0a0"
	s.Status.Gatus = "https://status.example.org"
	s.Status.Interval = "30s"
	if err := validateSite(s, map[string]string{}); err != nil {
		t.Fatalf("refused a legal config: %v", err)
	}
}
