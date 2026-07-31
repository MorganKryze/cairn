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
		// The sub-path trap: writing the address of the site rather than the
		// domain doubles the prefix in every canonical link and sitemap entry,
		// and only a crawler would ever notice.
		{"site url carrying the sub-path", func(s *Site) { s.URL = "https://example.org/cairn" },
			[]string{"url", "domain alone", "-base-path"}},
		{"site url carrying any path at all", func(s *Site) { s.URL = "https://example.org/a/b" },
			[]string{"domain alone"}},
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
		// The six fields an executable scheme used to reach. Every one is an
		// error rather than a warning, unlike a missing image file: a typo puts
		// a real path in the wrong place, and nothing puts javascript: there by
		// accident. The favicon is the one that escapes html/template entirely,
		// since it reaches the manifest as JSON and /favicon.ico as a Location.
		{"logo that is a script url", func(s *Site) { s.Logo = "javascript:alert(1)" },
			[]string{"logo", `"javascript:alert(1)"`, "javascript: scheme", "/assets"}},
		{"favicon that is a script url", func(s *Site) { s.Favicon = "vbscript:msgbox(1)" },
			[]string{"favicon", `"vbscript:msgbox(1)"`, "vbscript: scheme"}},
		// data:text/html carries a document, script and all. It is refused for
		// the same reason and named separately, because an operator who reaches
		// for it is reaching for an inline icon and deserves to be told which
		// of the three rules they hit.
		{"favicon that is a data url", func(s *Site) { s.Favicon = "data:text/html,<script>x</script>" },
			[]string{"favicon", "data: scheme"}},
		{"links url that is a script url", func(s *Site) {
			s.Links = []FooterLink{{Label: LString{"": "Wiki"}, URL: "javascript:alert(1)"}}
		}, []string{"links url", `"javascript:alert(1)"`, "mailto:"}},
		{"footer url that is a script url", func(s *Site) {
			s.Footer = []FooterLink{{Label: LString{"": "Legal"}, URL: "javascript:alert(1)"}}
		}, []string{"footer url", `"javascript:alert(1)"`}},
		{"footer icon that is a script url", func(s *Site) {
			s.Footer = []FooterLink{{Label: LString{"": "Legal"}, URL: "https://e.example.org", Icon: "javascript:alert(1)"}}
		}, []string{"footer icon", `"javascript:alert(1)"`, "glyph"}},
		// uriRe accepts any scheme, because security.txt takes mailto:, https:
		// and tel: alike, so this is the one URL field that blessed an
		// executable scheme outright instead of merely failing to look.
		{"security contact that is a script url", func(s *Site) { s.Security.Contact = "javascript:alert(1)" },
			[]string{"security.contact", `"javascript:alert(1)"`, "mailto:"}},
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

// A CSS hex colour has 3, 4, 6 or 8 digits and nothing in between. 5 and 7
// used to pass here, and a custom property whose value is not a valid colour
// is dropped at computed-value time: --accent falls back to nothing, and every
// declaration built on it goes with it, including the focus rings. The site
// still renders, so only this check can catch it.
func TestAccentTakesOnlyTheRealHexLengths(t *testing.T) {
	for _, c := range []struct {
		accent string
		ok     bool
	}{
		{"#abc", true},      // rgb
		{"#abcd", true},     // rgba
		{"#aabbcc", true},   // rrggbb
		{"#aabbccdd", true}, // rrggbbaa
		{"#AABBCC", true},   // case is not the browser's business
		{"#12345", false},   // the one that killed the focus rings
		{"#1234567", false}, // and its bigger sibling
		{"#ab", false},      // too short to be anything
		{"#123456789", false},
		{"aabbcc", false},  // no hash: a css identifier, not a colour
		{"#gghhii", false}, // hash but not hex
		{"", false},        // defaultSite always sets one, so empty means broken
	} {
		t.Run(c.accent, func(t *testing.T) {
			s := &Site{Locales: []string{"en"}}
			s.Theme.Accent = c.accent
			err := validateSite(s, map[string]string{})
			switch {
			case c.ok && err != nil:
				t.Errorf("accent %q was refused: %v", c.accent, err)
			case !c.ok && err == nil:
				t.Errorf("accent %q was accepted; it makes every var(--accent) rule invalid", c.accent)
			case !c.ok && !strings.Contains(err.Error(), "theme.accent"):
				t.Errorf("message %q does not name theme.accent", err)
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

// The plain spelling is the one nobody writes. A browser deletes every tab,
// newline and carriage return from a URL and strips the leading control
// characters and spaces before it reads the scheme, so all of the first group
// are one URL as far as it is concerned. A literal prefix test would refuse
// the obvious one and wave the rest through, which is worse than no check: it
// reads like a check that works.
func TestUnsafeSchemeSeesThroughTheDressing(t *testing.T) {
	for _, c := range []struct {
		name string
		val  string
		want string
	}{
		{"plain", "javascript:alert(1)", "javascript"},
		{"capitalised", "JavaScript:alert(1)", "javascript"},
		{"split by a tab", "java\tscript:alert(1)", "javascript"},
		{"split by a newline", "java\nscript:alert(1)", "javascript"},
		{"split by a carriage return", "java\rscript:alert(1)", "javascript"},
		{"led by spaces", "  javascript:alert(1)", "javascript"},
		{"led by a control character", "\x01javascript:alert(1)", "javascript"},
		{"vbscript, the same trick", "VB\tScript:msgbox(1)", "vbscript"},
		// data: is the third because data:text/html is a document with script
		// in it. The dressing works on it just the same.
		{"a data document", "data:text/html,<script>alert(1)</script>", "data"},
		{"a data image, refused all the same", "data:image/svg+xml,<svg/>", "data"},
		{"data, dressed", " Da\tta:text/html,x", "data"},
		// The look-alikes. Refusing one of these would be the worse failure of
		// the two: a config that booted yesterday stops booting on an upgrade,
		// and the operator is told their perfectly good path runs code.
		{"a file whose name contains the word", "/assets/javascript-logo.png", ""},
		{"a mailbox that contains the word", "mailto:javascript@example.org", ""},
		{"a url whose path contains it", "https://example.org/javascript:alert(1)", ""},
		{"a file whose name starts with the third word", "/assets/database.png", ""},
		{"a host that starts with it", "https://data.example.org/logo.png", ""},
		{"an ordinary asset path", "/assets/logo.png", ""},
		{"nothing at all", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := unsafeScheme(c.val); got != c.want {
				t.Errorf("unsafeScheme(%q) = %q, want %q", c.val, got, c.want)
			}
		})
	}
}

// The fields an executable scheme could never have reached, with the gate that
// stops each one. Nothing was added for them: a second refusal would only say
// the same thing twice. They are pinned here so that loosening one of those
// gates shows up as a hole in this rule rather than as a value in the manifest.
func TestUnsafeSchemeWasAlreadyRefusedWhereAGateExisted(t *testing.T) {
	for _, c := range []struct {
		name, want string
		mut        func(*Site)
	}{
		{"icons src, by the URL-or-/assets rule", "not a URL or an /assets path", func(s *Site) {
			s.Icons = []SiteIcon{{Src: "javascript:alert(1)", Sizes: "512x512"}}
		}},
		{"links icon, by the glyph rule", "is not a built-in glyph", func(s *Site) {
			s.Links = []FooterLink{{Label: LString{"": "W"}, URL: "https://w.example.org", Icon: "javascript:alert(1)"}}
		}},
		{"status.gatus, by the http rule", "status.gatus", func(s *Site) { s.Status.Gatus = "javascript:alert(1)" }},
		{"status.page, by the http rule", "status.page", func(s *Site) { s.Status.Page = "javascript:alert(1)" }},
		{"url, by the http rule", "url", func(s *Site) { s.URL = "javascript:alert(1)" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Site{Locales: []string{"en"}}
			s.Theme.Accent = "#247b7b"
			c.mut(s)
			err := validateSite(s, map[string]string{})
			if err == nil {
				t.Fatal("an executable scheme was accepted")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("message %q does not mention %q", err, c.want)
			}
		})
	}
}

// The shapes that must pass, so a tightened rule cannot quietly start
// refusing a config that has always been legal.
func TestValidateSiteAccepts(t *testing.T) {
	s := &Site{
		Locales: []string{"fr", "en", "pt-BR"},
		URL:     "http://tools.example.org",
		Logo:    "/assets/logo.png",
		Favicon: "https://cdn.example.org/favicon.png",
		Links: []FooterLink{
			{Label: LString{"": "Wiki"}, URL: "https://w.example.org", Icon: "book"},
			{Label: LString{"": "Logo"}, URL: "https://x.example.org", Icon: "/assets/l.png"},
			{Label: LString{"": "Plain"}, URL: "https://y.example.org"},
			// mailto: is a legal link target, and the scheme rule has to let
			// every scheme but the three named ones through.
			{Label: LString{"": "Mail"}, URL: "mailto:hi@example.org"},
		},
		Footer: []FooterLink{
			{Label: LString{"": "Legal"}, URL: "/en/legal/"},
			{Label: LString{"": "Status"}, URL: "https://status.example.org"},
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
