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
		// theme.font.family is inlined verbatim into the page's stylesheet,
		// so the refusal is about characters, not about looks: a semicolon or
		// a brace could break out of the declaration the value sits in.
		{"font family carrying a semicolon", func(s *Site) {
			s.Theme.Font.Family = "Inter; } body{display:none"
		}, []string{"theme.font.family", "Inter, system-ui"}},
		{"font family carrying a brace", func(s *Site) {
			s.Theme.Font.Family = "Inter {"
		}, []string{"theme.font.family", "Inter, system-ui"}},
		// The @font-face declares the first family of the list, and an
		// unquoted name with a space reads as two families, so the custom font
		// would be declared and never used. The message teaches the quote.
		{"font family whose first name has a space unquoted", func(s *Site) {
			s.Theme.Font.Family = "My Font, sans-serif"
		}, []string{"theme.font.family", "quoted", `"My Font"`}},
		{"font file without a family to name it", func(s *Site) {
			s.Theme.Font.File = "fonts/custom-font.woff2"
		}, []string{"theme.font.file", "theme.font.family"}},
		{"font file that is a URL", func(s *Site) {
			s.Theme.Font.Family = "Inter"
			s.Theme.Font.File = "https://cdn.example.org/font.woff2"
		}, []string{"theme.font.file", "fonts/ folder"}},
		{"font file climbing out of the config", func(s *Site) {
			s.Theme.Font.Family = "Inter"
			s.Theme.Font.File = "fonts/../../etc/passwd"
		}, []string{"theme.font.file", "fonts/ folder"}},
		{"font file with no servable extension", func(s *Site) {
			s.Theme.Font.Family = "Inter"
			s.Theme.Font.File = "fonts/readme.txt"
		}, []string{"theme.font.file", ".woff2"}},
		{"gatus that is not a URL", func(s *Site) { s.Status.Gatus = "status.example.org" },
			[]string{"status.gatus", "is not a URL"}},
		{"status page that is not a URL", func(s *Site) { s.Status.Page = "nope" },
			[]string{"status.page", "is not a URL"}},
		{"interval that is not a duration", func(s *Site) { s.Status.Interval = "often" },
			[]string{"status.interval", "at least 5s"}},
		{"interval below the floor", func(s *Site) { s.Status.Interval = "1s" },
			[]string{"status.interval", "at least 5s"}},
		// status.ca names a trust anchor. A value cairn cannot fetch or open
		// would fail on the first poll and take the pills with it, so it is
		// refused at the file rather than discovered a minute later in a log.
		{"ca that is neither a URL nor an assets path", func(s *Site) { s.Status.CA = "ca-bundle.crt" },
			[]string{"status.ca", "neither a URL nor a file", "/assets/"}},
		{"ca pointing outside the assets mount", func(s *Site) { s.Status.CA = "/assets/../etc/passwd" },
			[]string{"status.ca", "neither a URL nor a file"}},
		{"ca with an executable scheme", func(s *Site) { s.Status.CA = "javascript:alert(1)" },
			[]string{"status.ca", "javascript"}},
		{"ca as a file: URL", func(s *Site) { s.Status.CA = "file:///etc/ssl/ca.crt" },
			[]string{"status.ca", "file"}},
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
		// icon is a links key. A footer entry accepted it and never rendered
		// it, and schema/site.json has never listed it, so the editor refused
		// what the loader waved through. The value does not matter: any icon
		// at all is refused, hostile or not, because none of them render.
		{"footer entry with an icon", func(s *Site) {
			s.Footer = []FooterLink{{Label: LString{"": "Legal"}, URL: "https://e.example.org", Icon: "mail"}}
		}, []string{"footer entry", "has an icon", "move the entry to links"}},
		{"footer entry with a hostile icon, refused by the same rule", func(s *Site) {
			s.Footer = []FooterLink{{Label: LString{"": "Legal"}, URL: "https://e.example.org", Icon: "javascript:alert(1)"}}
		}, []string{"footer entry", "has an icon"}},
		// uriRe accepts any scheme, because security.txt takes mailto:, https:
		// and tel: alike, so this is the one URL field that blessed an
		// executable scheme outright instead of merely failing to look.
		{"security contact that is a script url", func(s *Site) { s.Security.Contact = "javascript:alert(1)" },
			[]string{"security.contact", `"javascript:alert(1)"`, "mailto:"}},
		// footer entries were validated nowhere at all. An entry with no url
		// renders an empty href, which is a footer link that looks live and
		// silently reloads the page; one with no label renders as nothing
		// visible. links has always demanded both.
		{"footer entry with no url", func(s *Site) {
			s.Footer = []FooterLink{{Label: LString{"": "Legal"}}}
		}, []string{"every footer entry needs label and url"}},
		// A hosting flag that leads nowhere is worse than one that leads
		// somewhere wrong: the visitor clicks and lands on a 404 of yours.
		{"a hosting flag naming a page that does not exist", func(s *Site) {
			s.Pages = []SitePage{{ID: "legal", Title: LString{"": "Legal"}, Body: LString{"": "text"}}}
			s.HostingFlag.Self = "hosting"
		}, []string{"hosting_flag.self", "hosting", "legal"}},
		{"a hosting flag with no page at all to name", func(s *Site) {
			s.HostingFlag.External = "why"
		}, []string{"hosting_flag.external", "why", "no pages"}},
		{"a hosting flag with an executable scheme", func(s *Site) {
			s.HostingFlag.Self = "javascript:alert(1)"
		}, []string{"hosting_flag.self", "javascript"}},
		{"footer entry with no label", func(s *Site) {
			s.Footer = []FooterLink{{URL: "/legal"}}
		}, []string{"every footer entry needs label and url"}},
		// The address resolution table. Every ambiguous way of naming the
		// monitor is answered with the key to fix and the shape expected,
		// because the alternative is cairn guessing which of two addresses the
		// operator meant and drawing pills from the wrong one.
		{"a provider nobody wrote", func(s *Site) {
			s.Status.URL = "https://x.example.org"
			s.Status.Provider = "nagios"
		}, []string{"status.provider", "nagios", "gatus"}},
		{"an address with no provider to read it", func(s *Site) {
			s.Status.URL = "https://k.example.org"
		}, []string{"status.url", "status.provider", "gatus"}},
		{"both addresses at once", func(s *Site) {
			s.Status.Gatus = "https://g.example.org"
			s.Status.URL = "https://k.example.org"
		}, []string{"status.gatus", "status.url", "not both"}},
		{"an address that is not a URL", func(s *Site) {
			s.Status.URL = "k.example.org"
			s.Status.Provider = "gatus"
		}, []string{"status.url", "is not a URL"}},
		// status.gatus is the Gatus spelling and says so by existing. Naming
		// another monitor beside it is two answers to one question.
		{"the gatus key with another monitor named", func(s *Site) {
			s.Status.Gatus = "https://g.example.org"
			s.Status.Provider = "kuma"
		}, []string{"status.gatus", "status.url", "kuma"}},
		// Kuma serves statuses per published status page and has no endpoint
		// that lists them all, so without the slug there is nothing to ask for.
		{"kuma without the slug it reads by", func(s *Site) {
			s.Status.URL = "https://k.example.org"
			s.Status.Provider = "kuma"
		}, []string{"status.slug", "published status page"}},
		{"a slug on a monitor that has no use for one", func(s *Site) {
			s.Status.Gatus = "https://g.example.org"
			s.Status.Slug = "tools"
		}, []string{"status.slug", "kuma"}},
		// A mapping that silently does nothing is the class of bug -check
		// exists to report; here it can be an error while someone is reading.
		{"json without the mapping that is its whole configuration", func(s *Site) {
			s.Status.URL = "https://s.example.org/api/v2/summary.json"
			s.Status.Provider = "json"
		}, []string{"status.map", "list", "key", "state"}},
		{"json with half a mapping", func(s *Site) {
			s.Status.URL = "https://s.example.org/api/v2/summary.json"
			s.Status.Provider = "json"
			s.Status.Map = StatusMap{List: "components", Key: "name"}
		}, []string{"status.map.state"}},
		{"a mapping on a monitor that has no use for one", func(s *Site) {
			s.Status.Gatus = "https://g.example.org"
			s.Status.Map = StatusMap{List: "components"}
		}, []string{"status.map", "json"}},
		// The one that publishes a secret rather than storing it. Verified:
		// GET /assets/token.txt on a running cairn returns the file.
		{"a token inside the served assets directory", func(s *Site) {
			s.Status.URL = "https://s.example.org"
			s.Status.Provider = "json"
			s.Status.Map = StatusMap{List: "c", Key: "n", State: "s", Up: []string{"ok"}}
			s.Status.TokenFile = "/assets/token.txt"
		}, []string{"status.token_file", "/assets", "every visitor"}},
		{"a token named by a relative path", func(s *Site) {
			s.Status.Gatus = "https://g.example.org"
			s.Status.TokenFile = "token.txt"
		}, []string{"status.token_file", "absolute path"}},
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

// The accepted half of the address resolution table, read through the two
// methods everything else asks rather than through the fields. A config
// written before providers existed has to come out of it saying gatus, since
// that is the shape every running site has.
func TestTheAddressResolves(t *testing.T) {
	for _, c := range []struct {
		name             string
		mut              func(*Site)
		provider, addr   string
		wantErrSubstring string
	}{
		{"gatus alone, which is every site running today",
			func(s *Site) { s.Status.Gatus = "https://g.example.org" }, "gatus", "https://g.example.org", ""},
		{"gatus named as well, which changes nothing",
			func(s *Site) {
				s.Status.Gatus = "https://g.example.org"
				s.Status.Provider = "gatus"
			}, "gatus", "https://g.example.org", ""},
		{"the generic address with its provider",
			func(s *Site) {
				s.Status.URL = "https://g.example.org"
				s.Status.Provider = "gatus"
			}, "gatus", "https://g.example.org", ""},
		{"a monitor that is not gatus",
			func(s *Site) {
				s.Status.URL = "https://k.example.org"
				s.Status.Provider = "kuma"
				s.Status.Slug = "tools"
			}, "kuma", "https://k.example.org", ""},
		{"somebody else's status API, read through a mapping",
			func(s *Site) {
				s.Status.URL = "https://s.example.org/api/v2/summary.json"
				s.Status.Provider = "json"
				s.Status.Map = StatusMap{List: "components", Key: "name", State: "status",
					Up: []string{"operational"}}
				// A token named by the path of a file the platform mounts is
				// the shape status.ca already accepts, and the one a
				// kubernetes secret, a docker secret and a vault agent all
				// deliver.
				s.Status.TokenFile = "/run/secrets/status-token"
			}, "json", "https://s.example.org/api/v2/summary.json", ""},
		// A provider with nothing to poll is not an error: it is the same
		// dead key as status.page without an address, and -check reports it.
		{"a provider with no address at all",
			func(s *Site) { s.Status.Provider = "gatus" }, "gatus", "", ""},
		{"no status block at all",
			func(s *Site) {}, "", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Site{Locales: []string{"en"}}
			s.Theme.Accent = "#247b7b"
			c.mut(s)
			if err := validateSite(s, map[string]string{}); err != nil {
				t.Fatalf("refused a legal config: %v", err)
			}
			if got := s.StatusProvider(); got != c.provider {
				t.Errorf("provider = %q, want %q", got, c.provider)
			}
			if got := s.StatusAddress(); got != c.addr {
				t.Errorf("address = %q, want %q", got, c.addr)
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

// The three documented spellings of a font file mean the same file and the
// same URL. They are the media/ dual spelling with a third shape: the served
// URL, which is what an operator copying it out of the page would write.
func TestFontRefAcceptsEverySpellingOfTheSameFile(t *testing.T) {
	for _, c := range []struct {
		file string
		rel  string
		ok   bool
	}{
		{"fonts/custom-font.woff2", "custom-font.woff2", true},  // the documented form
		{"custom-font.woff2", "custom-font.woff2", true},        // relative to fonts/
		{"/fonts/custom-font.woff2", "custom-font.woff2", true}, // the served URL
		{"sub/font.woff2", "sub/font.woff2", true},
		{"", "", false},
		{"https://cdn.example.org/font.woff2", "", false},
		{"data:font/woff2;base64,d09", "", false},
		{"/etc/font.woff2", "", false},
		{"fonts/../media/x.woff2", "", false},
		{"fonts/", "", false},
	} {
		rel, ok := FontRef(c.file)
		if rel != c.rel || ok != c.ok {
			t.Errorf("FontRef(%q) = (%q, %v), want (%q, %v)", c.file, rel, ok, c.rel, c.ok)
		}
	}
}

// The family name the @font-face declares is the first of the list, quoted
// when it has to be, and a comma inside a quoted name is one name rather than
// two. FirstFontFamily is the split that respects the quote.
func TestFirstFontFamilyAndFaceName(t *testing.T) {
	for _, c := range []struct {
		family string
		first  string
		face   string
	}{
		{`Inter, system-ui, sans-serif`, "Inter", `"Inter"`},
		{`"Inter", system-ui, sans-serif`, `"Inter"`, `"Inter"`},
		{`'My Font', sans-serif`, `'My Font'`, `'My Font'`},
		{`"My, Font", sans-serif`, `"My, Font"`, `"My, Font"`},
		{`ui-serif, Georgia, serif`, "ui-serif", `"ui-serif"`},
		{"", "", `""`},
	} {
		if got := FirstFontFamily(c.family); got != c.first {
			t.Errorf("FirstFontFamily(%q) = %q, want %q", c.family, got, c.first)
		}
		if got := FontFaceName(c.family); got != c.face {
			t.Errorf("FontFaceName(%q) = %q, want %q", c.family, got, c.face)
		}
	}
}

// A legal font block loads: a system stack alone, and a custom file with the
// family that names it, in either of the accepted spellings.
func TestValidateSiteAcceptsLegalFontBlocks(t *testing.T) {
	for _, c := range []struct {
		name string
		mut  func(*Site)
	}{
		{"a system stack", func(s *Site) { s.Theme.Font.Family = "ui-sans-serif, system-ui, sans-serif" }},
		{"a custom file", func(s *Site) {
			s.Theme.Font.Family = `"Inter", system-ui, sans-serif`
			s.Theme.Font.File = "fonts/custom-font.woff2"
		}},
		{"a custom file in the served-URL spelling", func(s *Site) {
			s.Theme.Font.Family = `Inter`
			s.Theme.Font.File = "/fonts/custom-font.woff2"
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Site{Locales: []string{"en"}}
			s.Theme.Accent = "#247b7b"
			c.mut(s)
			if err := validateSite(s, map[string]string{}); err != nil {
				t.Fatalf("refused a legal config: %v", err)
			}
		})
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
// characters and spaces before it reads the scheme, so every dressed spelling
// below is one URL as far as it is concerned. A literal prefix test would
// refuse the obvious one and wave the rest through, which is worse than no
// check: it reads like a check that works.
func TestURLSchemeReadsWhatABrowserReads(t *testing.T) {
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
		{"a data document", "data:text/html,<script>alert(1)</script>", "data"},
		{"data, dressed", " Da\tta:text/html,x", "data"},
		{"the one this rule was tightened for", "tel:+33123456789", "tel"},
		// The schemes cairn does emit read the same way; the allow-list, not
		// this function, is what decides.
		{"https", "https://tools.example.org", "https"},
		{"mailto", "mailto:hi@example.org", "mailto"},
		// A path has no scheme, and neither has a colon that arrives after the
		// path has started. Reading one of these as a scheme would refuse a
		// config that has always been legal and tell the operator their file
		// name is a protocol.
		{"a root-absolute path", "/assets/logo.png", ""},
		{"a path whose name contains a colon", "/assets/a:b.png", ""},
		{"a bare file name", "logo.png", ""},
		{"a url whose path contains a scheme", "https://example.org/javascript:alert(1)", "https"},
		{"a query that contains one", "https://example.org/?to=mailto:x@y.org", "https"},
		{"a fragment that contains one", "/legal#tel:0", ""},
		{"nothing at all", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := urlScheme(c.val); got != c.want {
				t.Errorf("urlScheme(%q) = %q, want %q", c.val, got, c.want)
			}
		})
	}
}

// tel: is the case that made this an allow-list rather than a list of the
// dangerous ones. It is nobody's attack, and it has never worked: html/template
// blanks it into a dead link exactly the way it blanks javascript:. The one
// field that keeps it is security.txt, which is plain text rather than markup,
// so nothing blanks anything and RFC 9116 names tel: itself.
func TestTelIsRefusedInLinksAndKeptInSecurityTxt(t *testing.T) {
	for _, c := range []struct {
		name string
		mut  func(*Site)
	}{
		{"a header link", func(s *Site) {
			s.Links = []FooterLink{{Label: LString{"": "Call"}, URL: "tel:+33123456789"}}
		}},
		{"a footer link", func(s *Site) {
			s.Footer = []FooterLink{{Label: LString{"": "Call"}, URL: "tel:+33123456789"}}
		}},
		{"a logo", func(s *Site) { s.Logo = "tel:+33123456789" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Site{Locales: []string{"en"}}
			s.Theme.Accent = "#247b7b"
			c.mut(s)
			err := validateSite(s, map[string]string{})
			if err == nil {
				t.Fatal("a tel: link was accepted, and it would have rendered dead")
			}
			if !strings.Contains(err.Error(), "tel: scheme") {
				t.Errorf("message %q does not name the scheme", err)
			}
		})
	}

	s := &Site{Locales: []string{"en"}}
	s.Theme.Accent = "#247b7b"
	s.Security.Contact = "tel:+33123456789"
	if err := validateSite(s, map[string]string{}); err != nil {
		t.Errorf("security.contact refused a scheme RFC 9116 names: %v", err)
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

	// The three shapes status.ca takes. http is deliberate, not an oversight:
	// an internal bundle often lives on a plain host, and a CA URL that has to
	// be https cannot be served by a CA nobody trusts yet.
	for _, ca := range []string{
		"https://pki.example.org/ca.crt",
		"http://pki.internal/ca.crt",
		"/assets/ca.crt",
	} {
		s.Status.CA = ca
		if err := validateSite(s, map[string]string{}); err != nil {
			t.Errorf("refused status.ca %q: %v", ca, err)
		}
	}
}
