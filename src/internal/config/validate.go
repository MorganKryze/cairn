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
	// engines act on it, so this refuses to boot rather than warn.
	if u, err := url.Parse(site.URL); site.URL != "" && err == nil && strings.Trim(u.Path, "/") != "" {
		return fmt.Errorf("config: site.yaml: url %q must be the domain alone, with no path (serving under example.org/cairn is what -base-path is for, and cairn adds that prefix itself)", site.URL)
	}
	for _, l := range site.Links {
		if len(l.Label) == 0 || l.URL == "" {
			return fmt.Errorf("config: site.yaml: every links entry needs label and url (expected: - {label: Wiki, url: https://…})")
		}
		if ic := l.Icon; ic != "" && !IsURLOrAbs(ic) {
			if _, ok := Glyphs[ic]; !ok {
				return fmt.Errorf("config: site.yaml: links icon %q is not a built-in glyph (%s), a URL or an /assets path", ic, strings.Join(GlyphNames(), ", "))
			}
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
