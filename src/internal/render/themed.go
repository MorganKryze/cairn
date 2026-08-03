package render

import (
	"fmt"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// Themed artwork: the second image a monochrome logo or icon needs, and how it
// reaches the page.
//
// The markup carries the light one and the page's stylesheet overrides it with
// `content: url(…)`, which is the only one of the three obvious techniques
// that is both cheap and correct here. Measured on Chromium and WebKit:
//
//   - two <img>, one hidden with display:none: BOTH are fetched, always, in
//     every theme. The page pays for artwork nobody can see.
//   - <picture> with media="(prefers-color-scheme: dark)": one fetch, but it
//     answers to the operating system and nothing else. cairn has a theme
//     button, so a visitor who switches by hand keeps the wrong artwork on
//     screen, which is worse than not offering the feature at all.
//   - content: url() in a rule: it follows data-theme, because it is CSS, and
//     the <img> keeps its alt, its width and height and its lazy loading.
//
// What that last one costs, measured rather than assumed: a visitor whose
// system is light fetches the light file alone, and the dark one only if they
// press the button. A visitor whose system is dark fetches both, because the
// src attribute starts loading before any rule applies and the rule then asks
// for the second. It is one extra file per themed image, for the half of
// visitors on a dark system, and it buys lazy loading and markup that still
// means something with no stylesheet. Serving the dark one from src instead
// would only move the cost onto the other half.
//
// The rules live in the inline <style> because they are per-site values, and a
// style attribute is not an option: the CSP allows the inline block by hash,
// and a hash never covers an attribute.
type themedSet struct {
	// class per resolved dark URL rather than per service: a site whose twenty
	// cards share one icon carries one rule.
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

// rules writes the dark overrides, twice over: once for the visitors whose
// system asks for dark and who have expressed no preference here, once for the
// ones who pressed the button. That is the pair style.css already uses for the
// theme toggle's own glyphs, and it is what makes the button work at all.
func (s *themedSet) rules(logoDark string) string {
	// No capacity hint: the list is one entry per distinct dark image on the
	// whole site, and len()+1 inside an allocation is what CodeQL reads as a
	// possible overflow. It is right that the shape is worth avoiding, and the
	// hint was buying nothing here.
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

// classFor is the class a card wears, empty for an icon that is one image. It
// reads the set rather than adding to it, so a card can never carry a class no
// rule paints.
func (s *themedSet) classFor(cfg *config.Config, ref config.ThemedRef) string {
	if !ref.Themed() {
		return ""
	}
	return s.class[AppURL(config.IconURL(cfg, ref.Dark))]
}

// themedFor hands back the set of dark images a config asks for.
//
// It is called twice, once to name the classes the cards wear and once to
// write the rules that paint them, and the two have to agree or a card wears
// a class nothing paints. They do, because this walks slices in config order
// and nothing here reads a map: one function, one input, one answer. Keep it
// that way; iterating a map to build the set would make the classes disagree
// on some runs and not others, which is the worst shape a bug can take.
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
