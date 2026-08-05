package render

import (
	"fmt"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// The second image a monochrome logo or icon needs. The markup carries the
// light one and a rule overrides it with content: url(), which follows
// data-theme and leaves the <img> its alt, its dimensions and its lazy
// loading. The two alternatives were measured on Chromium and WebKit and both
// fail here: two <img> with one display:none fetches both in every theme, and
// <picture> with a prefers-color-scheme media answers to the system alone, so
// the theme button would leave the wrong artwork on screen.
//
// The cost, also measured: a visitor on a dark system fetches both files,
// since src starts loading before any rule applies. One extra file per themed
// image for half of visitors. Serving dark from src moves that onto the others.
//
// The rules live in the inline <style> because they are per-site values, and
// the CSP allows that block by hash. A hash never covers a style attribute.
type themedSet struct {
	// keyed by resolved dark URL, not by service: twenty cards sharing one
	// icon carry one rule
	class map[string]string
	order []string
}

func newThemedSet() *themedSet { return &themedSet{class: map[string]string{}} }

// add records a dark image and returns the class that carries it.
func (s *themedSet) add(darkURL string) string {
	if c, ok := s.class[darkURL]; ok {
		return c
	}
	c := fmt.Sprintf("themed-%d", len(s.order)+1)
	s.class[darkURL] = c
	s.order = append(s.order, darkURL)
	return c
}

// rules writes the dark overrides twice: once for a dark system with no
// preference expressed here, once for the button. style.css uses the same pair
// for the toggle's own glyphs, and without both the button does nothing.
func (s *themedSet) rules(logoDark string) string {
	// no capacity hint: len()+1 inside an allocation is what CodeQL reads as a
	// possible overflow, and one entry per distinct dark image is a short list
	var sel []string
	if logoDark != "" {
		sel = append(sel, fmt.Sprintf(".brand img{content:url(%q)}", logoDark))
	}
	for _, u := range s.order {
		sel = append(sel, fmt.Sprintf(".%s{content:url(%q)}", s.class[u], u))
	}
	if len(sel) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("@media(prefers-color-scheme:dark){")
	for _, r := range sel {
		b.WriteString(`:root:not([data-theme="light"]) `)
		b.WriteString(r)
	}
	b.WriteString("}")
	for _, r := range sel {
		b.WriteString(`:root[data-theme="dark"] `)
		b.WriteString(r)
	}
	return b.String()
}

// classFor is the class a card wears, empty for a single-image icon. It reads
// the set rather than adding to it, so a card cannot carry an unpainted class.
func (s *themedSet) classFor(cfg *config.Config, ref config.ThemedRef) string {
	if !ref.Themed() {
		return ""
	}
	return s.class[AppURL(config.IconURL(cfg, ref.Dark))]
}

// themedFor is called twice, once to name the classes and once to write the
// rules, and the two must agree. They do because it walks slices in config
// order and reads no map. Building the set from a map would make the classes
// disagree on some runs and not others.
func themedFor(cfg *config.Config) *themedSet {
	s := newThemedSet()
	for _, c := range cfg.Categories {
		for _, sv := range c.Services {
			if sv.Icon.Themed() {
				s.add(AppURL(config.IconURL(cfg, sv.Icon.Dark)))
			}
		}
	}
	return s
}
