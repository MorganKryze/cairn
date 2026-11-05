package render_test

import (
	"strings"
	"testing"
)

func TestACardsBadgeOpensItsNote(t *testing.T) {
	home := detailPage(t, "[en]", betaService, "en")
	// The badge names the note twice: popovertarget opens it, aria-describedby
	// reads it after the word. Both point at the one id, or the note a click
	// shows is not the one a screen reader hears.
	want := `<button type="button" class="card-badge card-badge-beta" popovertarget="note-beta" aria-describedby="note-beta">Beta</button>` +
		`<span id="note-beta" class="state-note" popover>Available, but still being tested: it may change.</span>`
	if !strings.Contains(home, want) {
		t.Fatalf("the card does not carry its badge and note as %s", want)
	}
	// Outside .card-main, which is the text the search reads, so a word from
	// the note does not match every beta service.
	if strings.Index(home, `class="state-note"`) > strings.Index(home, `class="card-main"`) {
		t.Error("the note is written inside the card body, where the search reads it")
	}
}

func TestACardsNoteIsTheServicesOwn(t *testing.T) {
	home := detailPage(t, "[en]",
		"- {id: beta, url: 'https://beta.example.org', name: Beta one, state: beta, state_note: Ask Sam before relying on it.}\n",
		"en")
	if !strings.Contains(home, `class="state-note" popover>Ask Sam before relying on it.</span>`) {
		t.Error("the service's own state_note is not the card's note")
	}
	if strings.Contains(home, "still being tested") {
		t.Error("the built-in note is still on a card whose service replaced it")
	}
}

func TestACardsNoteInAnotherLanguageIsMarkedAsSuch(t *testing.T) {
	home := detailPage(t, "[en, fr]",
		"- {id: beta, url: 'https://beta.example.org', name: Beta one, state: beta, state_note: {en: Ask Sam first.}}\n",
		"fr")
	if !strings.Contains(home, `class="state-note" popover lang="en">Ask Sam first.</span>`) {
		t.Error("an English state_note on the French page is not marked lang=\"en\", so a screen reader reads it in French")
	}
}
