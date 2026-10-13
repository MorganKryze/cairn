package render

import (
	"regexp"
	"strings"
	"testing"
)

// The fold is a <details> around the note, so a phone sees one row and opens
// it without JavaScript. The row and the dismiss button are cairn strings, in
// the page's language; the note keeps its own language mark inside the fold.
// The button follows the fold so that Tab meets the row first, which is the
// order a phone shows them in.
func TestTheWelcomeNoteFoldsBehindARowInThePagesLanguage(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [fr, en]\nabout: {en: \"Welcome here.\"}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	home := string(m.Pages["fr"].HTML)

	start := strings.Index(home, `<section class="about">`)
	if start < 0 {
		t.Fatal(`the French home page has no <section class="about">`)
	}
	end := strings.Index(home[start:], "</section>")
	if end < 0 {
		t.Fatal("the welcome note's section is never closed")
	}
	section := home[start : start+end]

	fold := strings.Index(section, `<details class="about-fold"><summary class="about-sum">`)
	if fold < 0 {
		t.Fatalf("the note is not folded behind a row:\n%s", section)
	}
	sum := section[fold:]
	sum = sum[:strings.Index(sum, "</summary>")+len("</summary>")]
	if !strings.Contains(sum, "<span>À propos de ce site</span>") {
		t.Errorf("the row does not read the page's own words: %s", sum)
	}
	if strings.Contains(sum, "lang=") {
		t.Errorf("the row is a cairn string and carries no language mark: %s", sum)
	}

	note := strings.Index(section, `<div lang="en"><p>Welcome here.</p>`)
	closing := strings.Index(section, "</details>")
	button := strings.Index(section, `<button class="about-x"`)
	switch {
	case note < 0:
		t.Errorf("the English note lost its language mark:\n%s", section)
	case closing < 0:
		t.Errorf("the fold is never closed:\n%s", section)
	case button < 0:
		t.Errorf("the dismiss button left the section:\n%s", section)
	case note < fold || closing < note || button < closing:
		t.Errorf("want the row, the note inside the fold, then the button; got row %d, note %d, </details> %d, button %d",
			fold, note, closing, button)
	}
}

// about.summary is an ordinary string: an operator who words the row their own
// way sees it on the page, with the chevron after the words.
func TestTheFoldRowReadsAnOperatorsOwnWords(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml": "locales: [en]\nabout: \"Welcome here.\"\n" +
			"strings: {about.summary: {en: \"Read me first\"}}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	home := string(m.Pages["en"].HTML)
	if !strings.Contains(home, `<span>Read me first</span><svg class="about-chev"`) {
		t.Error("the row does not read about.summary, followed by the chevron")
	}
}

func TestASiteWithNoNoteHasNoFold(t *testing.T) {
	m := modelFrom(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	home := string(m.Pages["en"].HTML)
	for _, unwanted := range []string{"about-fold", "about-sum", "about-x"} {
		if strings.Contains(home, unwanted) {
			t.Errorf("a site with no welcome note still renders %s", unwanted)
		}
	}
}

// Hiding the row is only safe where the browser can also show the fold's
// content while it is closed, which is what ::details-content is for. A
// browser without it would hide the row and the note together, so every rule
// that hides the row has to sit inside the feature query.
func TestTheRowIsHiddenOnlyWhereTheNoteCanShow(t *testing.T) {
	raw, err := Embedded.ReadFile("assets/style.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(raw), "")
	hidesNone := regexp.MustCompile(`display\s*:\s*none`)

	var (
		stack []string // the at-rule preludes around the current position
		mark  int      // where the current prelude starts
		found int
	)
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case ';':
			mark = i + 1
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			mark = i + 1
		case '{':
			prelude := strings.TrimSpace(css[mark:i])
			if strings.HasPrefix(prelude, "@") {
				stack = append(stack, prelude)
				mark = i + 1
				continue
			}
			end := strings.IndexByte(css[i:], '}')
			if end < 0 {
				t.Fatalf("style.css: the rule %q is never closed", prelude)
			}
			body := css[i+1 : i+end]
			i += end
			mark = i + 1
			if !hidesRow(prelude) || !hidesNone.MatchString(body) {
				continue
			}
			found++
			guarded := false
			for _, at := range stack {
				if strings.HasPrefix(at, "@supports") && strings.Contains(at, "selector(::details-content)") {
					guarded = true
				}
			}
			if !guarded {
				t.Errorf("style.css hides the fold row outside @supports selector(::details-content): %s { %s }",
					prelude, strings.TrimSpace(body))
			}
		}
	}
	if found == 0 {
		t.Error("style.css never hides the fold row, so a wide screen shows a row above a note it cannot fold")
	}
}

// hidesRow says whether a selector list styles the row itself, as opposed to
// its marker or something inside it.
func hidesRow(prelude string) bool {
	for _, sel := range strings.Split(prelude, ",") {
		if strings.HasSuffix(strings.TrimSpace(sel), ".about-sum") {
			return true
		}
	}
	return false
}
