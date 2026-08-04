package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// Four services on purpose, because every check below is about telling them
// apart: one the host runs, one somebody else runs, one with nothing said, and
// one whose url is a page cairn serves itself.
func slPages(t *testing.T, siteExtra string) map[string]string {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml": "title: Test\nlocales: [en]\n" +
			"pages:\n  - id: hosting\n    title: Hosting\n    body: Where these run.\n" + siteExtra,
		"services.yaml": "- {id: pad, url: 'https://pad.example.org', name: Pad, desc: Ours., selfhosted: true}\n" +
			"- {id: mail, url: 'https://mail.example.org', name: Mail, desc: Theirs., selfhosted: false}\n" +
			"- {id: quiet, url: 'https://quiet.example.org', name: Quiet, desc: Unsaid.}\n" +
			"- {id: home, url: /en/hosting/, name: Hosting, desc: A page cairn serves.}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, map[string]status.State{})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, p := range m.Pages {
		out[k] = string(p.HTML)
	}
	return out
}

// Off by default: what a running site serves today has no target anywhere near
// a card name or the detail button.
func TestServiceLinksSilentWhenUnset(t *testing.T) {
	pages := slPages(t, "")
	if strings.Contains(pages["en"], `class="card-name" href="https://pad.example.org" target=`) {
		t.Error("a card name opened a new tab with no service_links block")
	}
	if strings.Contains(pages["en/pad"], `class="btn" href="https://pad.example.org" target=`) {
		t.Error("the detail button opened a new tab with no service_links block")
	}
	if strings.Contains(pages["en"], "data-leave") || strings.Contains(pages["en"], `id="leave"`) {
		t.Error("a leave dialog shipped with no service_links block")
	}
}

func TestNewTabOnlyForLinksThatLeaveTheSite(t *testing.T) {
	pages := slPages(t, "service_links:\n  new_tab: true\n")

	want := `class="card-name" href="https://pad.example.org" target="_blank" rel="noopener noreferrer"`
	if !strings.Contains(pages["en"], want) {
		t.Errorf("off-site card name did not open a new tab, want:\n  %s", want)
	}
	wantBtn := `class="btn" href="https://pad.example.org" target="_blank" rel="noopener noreferrer"`
	if !strings.Contains(pages["en/pad"], wantBtn) {
		t.Errorf("off-site detail button did not open a new tab, want:\n  %s", wantBtn)
	}

	// A service whose url is a path cairn serves is not leaving anything.
	if strings.Contains(pages["en"], `href="/en/hosting/" target=`) {
		t.Error("an in-site service link opened a new tab")
	}
	if strings.Contains(pages["en/home"], `class="btn" href="/en/hosting/" target=`) {
		t.Error("an in-site detail button opened a new tab")
	}
}

// `all` guards every link that leaves the site, whatever the card says about
// who runs the thing.
func TestConfirmAllGuardsEveryOffSiteLink(t *testing.T) {
	pages := slPages(t, "service_links:\n  confirm: all\n")

	for _, id := range []string{"pad", "mail", "quiet"} {
		if !strings.Contains(pages["en"], `href="https://`+id+`.example.org" data-leave`) {
			t.Errorf("%s is not guarded under confirm: all", id)
		}
	}
	if strings.Contains(pages["en"], `href="/en/hosting/" data-leave`) {
		t.Error("an in-site link was guarded")
	}
	if n := strings.Count(pages["en"], `id="leave"`); n != 1 {
		t.Errorf("home page carries %d leave dialogs, want 1", n)
	}
	if n := strings.Count(pages["en/pad"], `id="leave"`); n != 1 {
		t.Errorf("detail page carries %d leave dialogs, want 1", n)
	}
	if !strings.Contains(pages["en"], AssetURL("leave.js")) {
		t.Error("the page does not load leave.js")
	}
}

// The point of the scope: only the services flagged external raise it, and a
// service nobody has flagged is not assumed to be somebody else's.
func TestConfirmExternalGuardsOnlyTheFlaggedOnes(t *testing.T) {
	pages := slPages(t, "service_links:\n  confirm: external\n")

	if !strings.Contains(pages["en"], `href="https://mail.example.org" data-leave`) {
		t.Error("the external service is not guarded")
	}
	if strings.Contains(pages["en"], `href="https://pad.example.org" data-leave`) {
		t.Error("a self-hosted service was guarded")
	}
	if strings.Contains(pages["en"], `href="https://quiet.example.org" data-leave`) {
		t.Error("an unflagged service was guarded")
	}
	// The detail pages have to agree with the cards, or a visitor meets the
	// warning on one route and not the other.
	if !strings.Contains(pages["en/mail"], `class="btn" href="https://mail.example.org" data-leave`) {
		t.Error("the external detail button is not guarded")
	}
	if strings.Contains(pages["en/pad"], "data-leave") {
		t.Error("the self-hosted detail button was guarded")
	}
	// A page with nothing to guard carries no dialog and loads no script.
	if strings.Contains(pages["en/pad"], `id="leave"`) || strings.Contains(pages["en/pad"], AssetURL("leave.js")) {
		t.Error("a page with no guarded link still shipped the dialog")
	}
}

func TestNoDialogWithoutConfirm(t *testing.T) {
	pages := slPages(t, "service_links:\n  new_tab: true\n")
	if strings.Contains(pages["en"], `id="leave"`) {
		t.Error("a leave dialog shipped with confirm off")
	}
	if strings.Contains(pages["en"], AssetURL("leave.js")) {
		t.Error("leave.js shipped with confirm off")
	}
	if strings.Contains(pages["en"], "data-leave") {
		t.Error("a link was marked data-leave with confirm off")
	}
}

// The two compose, and the dialog's own continue link has to agree with
// new_tab, or a visitor who confirms lands somewhere the operator did not
// choose.
func TestConfirmAndNewTabAgree(t *testing.T) {
	pages := slPages(t, "service_links:\n  new_tab: true\n  confirm: all\n")
	if !strings.Contains(pages["en"], `class="card-name" href="https://pad.example.org" target="_blank" rel="noopener noreferrer" data-leave`) {
		t.Error("a confirmed link lost its new tab")
	}
	if !strings.Contains(pages["en"], `class="btn leave-go" href="" target="_blank" rel="noopener noreferrer"`) {
		t.Error("the dialog's continue link does not open a new tab")
	}
}

// Without new_tab the dialog's continue link navigates in place, the way the
// link it stands in for would have.
func TestConfirmWithoutNewTabNavigatesInPlace(t *testing.T) {
	pages := slPages(t, "service_links:\n  confirm: all\n")
	if !strings.Contains(pages["en"], `class="btn leave-go" href=""`) {
		t.Error("no continue link in the dialog")
	}
	if strings.Contains(pages["en"], `class="btn leave-go" href="" target=`) {
		t.Error("the continue link opened a new tab with new_tab off")
	}
}
