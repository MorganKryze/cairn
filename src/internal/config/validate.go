package config

import (
	"fmt"
	"net/url"
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
	// plausible typo. A script scheme is not, so it stops the load. The favicon
	// is the sharp case: it reaches the manifest as JSON, which no html/template
	// escaping covers, and comes back from /favicon.ico as a Location header.
	for _, f := range []struct{ key, val string }{
		{"logo", site.Logo},
		{"favicon", site.Favicon},
	} {
		if hasScriptScheme(f.val) {
			return scriptSchemeErr(f.key, f.val, "https://… or an /assets path")
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
		// and tel: alike, so it is the one URL check here that a script scheme
		// walks straight through. Catch it before uriRe has a chance to bless
		// it, or the researcher this file is written for gets a dead link.
		if hasScriptScheme(f.val) {
			return scriptSchemeErr(f.key, f.val, "e.g. mailto:security@example.org or https://example.org/security")
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
		// A link url is the one field here that takes any scheme at all, since
		// mailto: and tel: are legitimate targets, so nothing else would stop
		// this one. html/template blanks the href, which means the header link
		// renders and goes nowhere with no error anywhere.
		if hasScriptScheme(l.URL) {
			return scriptSchemeErr("links url", l.URL, "https://…, mailto:… or an absolute path")
		}
		if ic := l.Icon; ic != "" && !IsURLOrAbs(ic) {
			// A script scheme lands here too: it is not a URL, not an /assets
			// path and not a glyph name, so this refusal already covers it and
			// a second gate would only say the same thing twice.
			if _, ok := Glyphs[ic]; !ok {
				return fmt.Errorf("config: site.yaml: links icon %q is not a built-in glyph (%s), a URL or an /assets path", ic, strings.Join(GlyphNames(), ", "))
			}
		}
	}
	// Footer entries are validated nowhere else: they carry no icon on the page
	// and no rule ever asked what their url was. That silence is exactly why a
	// script scheme has to be caught here rather than left to the next reader.
	for _, l := range site.Footer {
		if hasScriptScheme(l.URL) {
			return scriptSchemeErr("footer url", l.URL, "https://…, mailto:… or an absolute path")
		}
		if hasScriptScheme(l.Icon) {
			return scriptSchemeErr("footer icon", l.Icon, "a built-in glyph name, https://… or an /assets path")
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

// hasScriptScheme reports a value a browser would read as javascript: or
// vbscript:, however it is dressed up.
//
// The dressing is the whole point. A browser removes every tab, newline and
// carriage return from a URL and strips leading control characters and spaces
// before it looks at the scheme, so "java\tscript:alert(1)", " javascript:…"
// and "JavaScript:…" are all the same URL as the plain one. Matching the
// literal text would refuse the obvious spelling and pass the three that
// matter, which is worse than not checking: it reads as a check that works.
func hasScriptScheme(s string) bool {
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	s = strings.ToLower(strings.TrimLeftFunc(s, func(r rune) bool { return r <= ' ' }))
	return strings.HasPrefix(s, "javascript:") || strings.HasPrefix(s, "vbscript:")
}

// scriptSchemeErr words the refusal once for every field that can carry one,
// so an operator who trips it twice can tell it is one rule and not two.
func scriptSchemeErr(field, val, accepted string) error {
	return fmt.Errorf("config: site.yaml: %s %q uses the javascript: or vbscript: scheme, which loads nothing (expected %s)", field, val, accepted)
}
