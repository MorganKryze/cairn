package config

import (
	"fmt"
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
	site.URL = strings.TrimSuffix(site.URL, "/")
	if u := site.URL; u != "" && !isHTTPURL(u) {
		return fmt.Errorf("config: site.yaml: url %q is not a URL (expected e.g. https://tools.example.org)", u)
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
