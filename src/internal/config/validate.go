package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// validateSite normalizes and checks the site.yaml fields (defaults, hex
// accent, URLs, links, pages). definedIn is the service-id -> file map, used
// to reject page ids that collide with a service id.
func validateSite(site *Site, definedIn map[string]string) error {
	if len(site.Locales) == 0 {
		site.Locales = []string{"en"}
	}
	for _, l := range site.Locales {
		if !localeRe.MatchString(l) {
			return fmt.Errorf("config: site.yaml: invalid locale %q (expected codes like fr, en, pt-BR)", l)
		}
	}
	if !accentRe.MatchString(site.Theme.Accent) {
		return fmt.Errorf("config: site.yaml: theme.accent %q is not a hex color (expected e.g. \"#247b7b\")", site.Theme.Accent)
	}
	if g := site.Status.Gatus; g != "" && !isHTTPURL(g) {
		return fmt.Errorf("config: site.yaml: status.gatus %q is not a URL (expected e.g. https://status.example.org)", g)
	}
	if p := site.Status.Page; p != "" && !isHTTPURL(p) {
		return fmt.Errorf("config: site.yaml: status.page %q is not a URL (expected e.g. https://status.example.org)", p)
	}
	if iv := site.Status.Interval; iv != "" {
		if d, err := time.ParseDuration(iv); err != nil || d < 5*time.Second {
			return fmt.Errorf("config: site.yaml: status.interval %q is not a duration of at least 5s (expected e.g. 60s)", iv)
		}
	}
	// logo and favicon are the two image fields nothing else validates: a value
	// that is not a URL only earns a -check warning, since a missing file is a
	// plausible typo. An executable scheme is not, so it stops the load. The
	// favicon is the sharp case: it reaches the manifest as JSON, which no
	// html/template escaping covers, and comes back from /favicon.ico as a
	// Location header.
	for _, f := range []struct{ key, val string }{
		{"logo", site.Logo},
		{"favicon", site.Favicon},
	} {
		if err := checkLinkScheme(f.key, f.val, imageSchemes, "https://… or an /assets path"); err != nil {
			return err
		}
	}
	// security.txt is a line-based format, so a newline in any value would let
	// a typo forge a field. Reject that here rather than escape it later.
	for _, f := range []struct{ key, val string }{
		{"security.contact", site.Security.Contact},
		{"security.policy", site.Security.Policy},
		{"security.encryption", site.Security.Encryption},
	} {
		if f.val == "" {
			continue
		}
		if strings.ContainsAny(f.val, "\r\n") {
			return fmt.Errorf("config: site.yaml: %s must be one line", f.key)
		}
		// uriRe takes any scheme, because security.txt takes mailto:, https:
		// and tel: alike, so it is the one URL check here that an executable
		// scheme walks straight through. Catch it before uriRe has a chance to
		// bless it, or the researcher this file is written for gets a dead
		// link. tel: stays legal here and nowhere else: this file is plain
		// text, so nothing blanks it, and RFC 9116 names it.
		if err := checkContactScheme(f.key, f.val); err != nil {
			return err
		}
		if !uriRe.MatchString(f.val) {
			return fmt.Errorf("config: site.yaml: %s %q is not a URI (expected e.g. mailto:security@example.org or https://example.org/security)", f.key, f.val)
		}
	}
	if site.Security.Contact == "" && (site.Security.Policy != "" || site.Security.Encryption != "") {
		return fmt.Errorf("config: site.yaml: security needs a contact (expected: security: {contact: mailto:security@example.org})")
	}
	site.URL = strings.TrimSuffix(site.URL, "/")
	if u := site.URL; u != "" && !isHTTPURL(u) {
		return fmt.Errorf("config: site.yaml: url %q is not a URL (expected e.g. https://tools.example.org)", u)
	}
	// url is the domain, never the address of the page. cairn appends
	// -base-path itself, so a path written here is carried twice: an operator
	// serving example.org/cairn who writes the full public URL ends up with
	// canonical links, hreflang alternates and a whole sitemap pointing at
	// example.org/cairn/cairn/. Nothing else would ever say so, and search
	// engines act on it, so this is an error rather than a warning: a fresh
	// start then serves the getting-started page with the reason logged, and a
	// reload keeps the previous pages. cairn never exits on a bad config.
	if u, err := url.Parse(site.URL); site.URL != "" && err == nil && strings.Trim(u.Path, "/") != "" {
		return fmt.Errorf("config: site.yaml: url %q must be the domain alone, with no path (serving under example.org/cairn is what -base-path is for, and cairn adds that prefix itself)", site.URL)
	}
	for _, l := range site.Links {
		if len(l.Label) == 0 || l.URL == "" {
			return fmt.Errorf("config: site.yaml: every links entry needs label and url (expected: - {label: Wiki, url: https://…})")
		}
		// mailto: is a legitimate target here, which is why this field gets the
		// link list rather than the image one.
		if err := checkLinkScheme("links url", l.URL, linkSchemes, "https://…, mailto:… or an absolute path"); err != nil {
			return err
		}
		if ic := l.Icon; ic != "" && !IsURLOrAbs(ic) {
			// Any scheme at all lands here: it is not a URL, not an /assets path
			// and not a glyph name, so this refusal already covers the ones
			// above and a second gate would only say the same thing twice.
			if _, ok := Glyphs[ic]; !ok {
				return fmt.Errorf("config: site.yaml: links icon %q is not a built-in glyph (%s), a URL or an /assets path", ic, strings.Join(GlyphNames(), ", "))
			}
		}
	}
	// Footer entries used to be validated nowhere at all: an entry with no url
	// rendered an empty href, which is a footer link that looks live and
	// reloads the page, and one with no label rendered as nothing visible at
	// all. links has always demanded both, and there was never a reason for
	// the two lists to disagree.
	for _, l := range site.Footer {
		if len(l.Label) == 0 || l.URL == "" {
			return fmt.Errorf("config: site.yaml: every footer entry needs label and url (expected: - {label: Legal, url: /legal})")
		}
		// icon is a header-link key. A footer entry took it without complaint
		// and never rendered it, which -check reported as inert; schema/site.json
		// has never listed it at all, so the editor said one thing and the
		// loader another. Refusing it is what makes those two agree, and it says
		// the useful half out loud: there is a list where the icon does work.
		if l.Icon != "" {
			return fmt.Errorf("config: site.yaml: footer entry %q has an icon: only header links render one (move the entry to links, or drop the icon)", l.URL)
		}
		if err := checkLinkScheme("footer url", l.URL, linkSchemes, "https://…, mailto:… or an absolute path"); err != nil {
			return err
		}
	}
	for i, ic := range site.Icons {
		where := fmt.Sprintf("icons entry %d", i+1)
		if ic.Src == "" || ic.Sizes == "" {
			return fmt.Errorf("config: site.yaml: %s needs src and sizes "+
				"(expected: - {src: /assets/icon-512.png, sizes: 512x512})", where)
		}
		if !IsURLOrAbs(ic.Src) {
			return fmt.Errorf("config: site.yaml: %s src %q is not a URL or an /assets path", where, ic.Src)
		}
		if !iconSizesRe.MatchString(ic.Sizes) {
			return fmt.Errorf("config: site.yaml: %s sizes %q is not a size "+
				"(expected e.g. 512x512, or \"any\" for an svg)", where, ic.Sizes)
		}
		for _, p := range strings.Fields(ic.Purpose) {
			if p != "any" && p != "maskable" && p != "monochrome" {
				return fmt.Errorf("config: site.yaml: %s purpose %q is not one of "+
					"any, maskable, monochrome (they may be combined: \"any maskable\")", where, p)
			}
		}
	}
	pageIDs := map[string]bool{}
	for _, p := range site.Pages {
		switch {
		case !idRe.MatchString(p.ID):
			return fmt.Errorf("config: site.yaml: invalid page id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. legal)", p.ID)
		case len(p.Title) == 0 || (len(p.Body) == 0 && len(p.Sections) == 0):
			return fmt.Errorf("config: site.yaml: page %q needs a title and a body or sections", p.ID)
		case pageIDs[p.ID]:
			return fmt.Errorf("config: site.yaml: duplicate page id %q", p.ID)
		case definedIn[p.ID] != "":
			return fmt.Errorf("config: site.yaml: page id %q collides with the service id defined in %s", p.ID, definedIn[p.ID])
		}
		pageIDs[p.ID] = true
		for _, s := range p.Sections {
			if len(s.Title) == 0 || len(s.Body) == 0 {
				return fmt.Errorf("config: site.yaml: page %q: every section needs title and body", p.ID)
			}
		}
	}
	return nil
}

// The schemes cairn will actually emit. html/template puts http, https,
// mailto and paths in an href or a src, and replaces every other scheme with
// #ZgotmplZ, so a value carrying one renders as a link that goes nowhere and
// says nothing. tel: fails exactly the way javascript: does, and just as
// quietly; the only difference between them is intent.
//
// An allow-list rather than a list of the dangerous ones. A deny-list is only
// as current as the last time somebody thought about it, and the question that
// decides this is not which schemes are hostile but which ones cairn can emit.
var (
	linkSchemes = []string{"http", "https", "mailto"}
	// An image is a file, so mailto: has no business naming one.
	imageSchemes = []string{"http", "https"}
	// security.txt is the exception, and the reason the allow-list above is
	// scoped to links rather than applied to the whole file: that file is
	// plain text, never markup, so nothing blanks anything on the way out and
	// RFC 9116 names tel: itself. Only the schemes a browser can be talked
	// into executing are refused there, and data: is among them because
	// data:text/html carries a whole document, script and all.
	unsafeSchemes = []string{"javascript", "vbscript", "data"}
)

// urlScheme returns the scheme a browser would read from s, lowercased, and ""
// when there is none, which is every relative and root-absolute path.
//
// It reads the value the way a browser does rather than the way it is written.
// A browser removes every tab, newline and carriage return from a URL and
// strips leading control characters and spaces before it looks at the scheme,
// so "java\tscript:alert(1)", " javascript:…" and "JavaScript:…" are all the
// same URL as the plain one. Matching the literal text would refuse the
// obvious spelling and pass the three that matter, which is worse than not
// checking: it reads as a check that works.
func urlScheme(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimLeftFunc(s, func(r rune) bool { return r <= ' ' })
	i := strings.IndexByte(s, ':')
	// A colon past a slash, a question mark or a hash belongs to the path or
	// the query: "/assets/a:b.png" is a path and always was.
	if i < 0 || strings.ContainsAny(s[:i], "/?#") {
		return ""
	}
	return strings.ToLower(s[:i])
}

// checkLinkScheme refuses a value cairn cannot put in an href or a src.
//
// It is an error rather than a warning because the alternative is what this
// replaces: the page renders, the link is there, it does nothing when clicked,
// and no log line anywhere mentions it.
func checkLinkScheme(field, val string, allowed []string, accepted string) error {
	sc := urlScheme(val)
	if sc == "" || slices.Contains(allowed, sc) {
		return nil
	}
	return fmt.Errorf("config: site.yaml: %s %q uses the %s: scheme, which cairn does not emit: the link would render dead and nothing would say why (expected %s)", field, val, sc, accepted)
}

// checkContactScheme is the security.txt half: any URI but an executable one.
func checkContactScheme(field, val string) error {
	if sc := urlScheme(val); slices.Contains(unsafeSchemes, sc) {
		return fmt.Errorf("config: site.yaml: %s %q uses the %s: scheme, which no researcher can act on (expected e.g. mailto:security@example.org, https://example.org/security or tel:+33…)", field, val, sc)
	}
	return nil
}
