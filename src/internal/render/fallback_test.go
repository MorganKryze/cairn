package render

import (
	"strings"
	"testing"
)

// A field with no translation for the page's locale still renders, in whatever
// language the config does have. Left unsaid, the page claims through
// <html lang> that the sentence is in its own language: a screen reader reads
// it in the wrong voice, and a right-to-left fallback inside a left-to-right
// page is laid out the wrong way round, which moves its punctuation to the
// wrong end of the line.
func TestFallbackTextCarriesTheLanguageItIsWrittenIn(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n" +
			"title: {fr: Mes outils, en: My tools}\n" +
			// English on a French page: both read left to right, so lang is
			// the whole of what is missing and dir would be noise.
			"tagline: {en: Everything in one place}\n" +
			"about: {ar: \"مرحبا بكم في هذا الموقع.\"}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, " +
			"desc: {ar: \"محرر نصوص تعاوني\"}, details: {ar: \"فقرة من النص.\"}}\n",
	})

	home := string(m.Pages["fr"].HTML)
	for _, want := range []string{
		`<p class="tagline" lang="en">Everything in one place</p>`,
		`<div lang="ar" dir="rtl">`,
		// The mark sits on the description itself. It used to be on an inner
		// span, because the "Learn more" words shared that element and were
		// written in the page's language; the link is a glyph in the corner
		// now, so the whole element is the description and carries the mark.
		`<span class="card-desc" lang="ar" dir="rtl">محرر نصوص تعاوني</span>`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("the French home page does not carry %s", want)
		}
	}
	// A plain YAML string is the operator saying one wording serves every
	// locale, which is what a brand name is. It is never a fallback.
	if !strings.Contains(home, `<a class="card-name" href="https://pad.example.org">Pad</a>`) {
		t.Error("a name written once for every locale should carry no mark")
	}
	if strings.Contains(home, `lang="en" dir=`) {
		t.Error("English text on a French page reads the same way round: dir should be left out")
	}

	detail := string(m.Pages["fr/pad"].HTML)
	for _, want := range []string{
		`<p class="lead" lang="ar" dir="rtl">محرر نصوص تعاوني</p>`,
		`<div lang="ar" dir="rtl"><p>فقرة من النص.</p>`,
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("the French detail page does not carry %s", want)
		}
	}
}

// The half that matters more. A site translated for the locales it lists must
// come out exactly as it did before any of this existed: one lang and one dir,
// on <html>, and not a single other. A mark on every field would say "another
// language" so often that the times it is true would stop meaning anything.
func TestTranslatedSiteCarriesNoLanguageMarks(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n" +
			"url: https://tools.example.org\n" +
			"title: {fr: Mes outils, en: My tools}\n" +
			"tagline: {fr: Tout au même endroit, en: Everything in one place}\n" +
			"about: {fr: \"Bienvenue.\\n\\nBonne visite.\", en: \"Welcome.\\n\\nEnjoy.\"}\n" +
			"links: [{label: {fr: Wiki, en: Wiki}, url: https://wiki.example.org}]\n" +
			"footer: [{label: {fr: Statut, en: Status}, url: https://status.example.org}]\n" +
			"pages:\n  - id: legal\n    title: {fr: Mentions légales, en: Legal notice}\n" +
			"    body: {fr: Éditeur, en: Publisher}\n" +
			"    sections:\n      - {title: {fr: Hébergeur, en: Host}, body: {fr: Ici, en: Here}}\n",
		"categories.yaml": "- {id: docs, name: {fr: Documents, en: Documents}}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, category: docs, " +
			"name: {fr: Bloc-notes, en: Pad}, desc: {fr: Éditeur, en: Editor}, " +
			"details: {fr: Un paragraphe., en: A paragraph.}, " +
			"images: [{src: 'https://img.example.org/shot.png', caption: {fr: Aperçu, en: Preview}}]}\n",
	})

	for path, page := range m.Pages {
		html := string(page.HTML)
		// hreflang="…" does not match: the space before lang is what makes
		// this count the attribute rather than the end of another word.
		if n := strings.Count(html, ` lang="`); n != 1 {
			t.Errorf("/%s/ carries %d lang attributes, want only the one on <html>", path, n)
		}
		if n := strings.Count(html, ` dir="`); n != 1 {
			t.Errorf("/%s/ carries %d dir attributes, want only the one on <html>", path, n)
		}
		// The wrapper the prose template adds exists only to carry a mark, so a
		// page with nothing to mark must not have grown one.
		if strings.Contains(html, "<div lang") || strings.Contains(html, "<span lang") {
			t.Errorf("/%s/ grew a wrapper element for a language it does not need", path)
		}
	}
}

// A regional variant is the same language read by the same voice: a site that
// writes pt for pt-BR is translated, not falling back, and must not collect a
// lang attribute on every field for the difference.
func TestRegionalVariantIsNotAFallback(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [pt-BR]\ntagline: {pt: Tudo num só lugar}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	home := string(m.Pages["pt-BR"].HTML)
	if !strings.Contains(home, `<p class="tagline">Tudo num só lugar</p>`) {
		t.Error("pt wording on a pt-BR page should render unmarked")
	}
}

// Every marked field is a struct now, and {{if .Field}} on a struct is always
// true: a template that shows an element only when there is text has to test
// .Text. This is what catches the slip, since the symptom is an empty element
// nobody looks at rather than a failure.
func TestEmptyFieldsStillRenderNothing(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n" +
			"- {id: shot, url: https://shot.example.org, name: Shot, images: ['https://img.example.org/shot.png']}\n",
	})
	home := string(m.Pages["en"].HTML)
	for _, unwanted := range []string{`class="tagline"`, `class="about"`} {
		if strings.Contains(home, unwanted) {
			t.Errorf("home rendered %s for a field the config never set", unwanted)
		}
	}
	// Neither service has a description, so neither has a card-desc. shot has
	// images and therefore a detail link, which used to be the second reason
	// for that element and is now a glyph of its own in the corner.
	if strings.Contains(home, `class="card-desc"`) {
		t.Error("a service with no desc should render no card-desc")
	}
	if strings.Count(home, `class="card-more"`) != 1 {
		t.Error("shot has a detail page to link to, pad has none: exactly one glyph")
	}
	detail := string(m.Pages["en/shot"].HTML)
	for _, unwanted := range []string{"<figcaption", `class="lead"`} {
		if strings.Contains(detail, unwanted) {
			t.Errorf("detail page rendered %s for a field the config never set", unwanted)
		}
	}
}

// langAttrs is the one place a value is written into a tag without the
// template escaper, so it checks rather than trusts. Config cannot reach it
// with any of this (a locale key that is not a locale code is refused at load),
// and if some later field found another road, the mark is dropped rather than
// letting a forged attribute through.
func TestForgedLocaleNeverBecomesAnAttribute(t *testing.T) {
	for _, bad := range []string{"", `fr" onmouseover="alert(1)`, "fr>", "fr ltr", "../ar", "fr'"} {
		if got := langAttrs(bad, "rtl"); got != "" {
			t.Errorf("langAttrs(%q, \"rtl\") = %q, want nothing at all", bad, got)
		}
	}
	if got := langAttrs("en", `ltr" onload="alert(1)`); got != ` lang="en"` {
		t.Errorf("a direction that is not ltr or rtl leaked: %q", got)
	}
	for _, c := range []struct{ lang, dir, want string }{
		{"ar", "rtl", ` lang="ar" dir="rtl"`},
		{"pt-BR", "", ` lang="pt-BR"`},
	} {
		if got := string(langAttrs(c.lang, c.dir)); got != c.want {
			t.Errorf("langAttrs(%q, %q) = %q, want %q", c.lang, c.dir, got, c.want)
		}
	}
}
