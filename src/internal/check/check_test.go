package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestCheckWarnings(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nabout: {fr: Bonjour}\n",
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: {fr: Outil, en: Tool}\n  desc: {fr: Une phrase}\n" +
			"- id: b\n  url: https://b.example.org\n  name: Partout\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "orphan.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{
		"site about has no en",
		`service "a" desc has no en`,
		"media/orphan.png is not referenced",
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	if strings.Contains(warnings, `"b"`) {
		t.Errorf("plain-string service should not warn:\n%s", warnings)
	}
}

func TestCheckMarkdownImageCountsAsUsed(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlogo: /assets/logo.png\nlocales: [en]\nabout: \"See ![plan](plan.png)\"\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "plan.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := checkWarnings(cfg, dir); len(w) != 0 {
		t.Errorf("warnings = %v, want none", w)
	}
}

// -check is what an operator wires into their own pipeline, so its warnings
// have to name the thing that is wrong and stay quiet about the rest.
func TestWarningsNameWhatIsMissing(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n" +
			"tagline: {fr: Seulement en français}\n" +
			"about: {fr: Bonjour, en: Hello}\n" +
			"pages: [{id: legal, title: {fr: Mentions}, body: {fr: Texte, en: Text}, " +
			"sections: [{title: {fr: Éditeur}, body: {fr: Nom, en: Name}}]}]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: {fr: Bloc, en: Pad}, desc: {fr: Écrire}}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")

	// each field covering some locales but not all is named
	for _, want := range []string{"tagline", "legal", "pad", "section 1 title"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings never mention %q:\n%s", want, warnings)
		}
	}
	// a field translated everywhere is not mentioned
	if strings.Contains(warnings, "about") {
		t.Errorf("a fully translated field was flagged:\n%s", warnings)
	}
}

// A config with nothing missing says so and warns about nothing, which is what
// makes the warnings worth reading when they do appear.
func TestCompleteConfigWarnsAboutNothing(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		// url and a raster logo: without them the canonical links and the
		// link-preview image are silently absent, and -check says so.
		"site.yaml":     "url: https://tools.example.org\nlogo: /assets/logo.png\nlocales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, desc: Write together.}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := checkWarnings(cfg, dir); len(w) != 0 {
		t.Errorf("a complete config produced warnings: %v", w)
	}
	if code := RunCheck(dir); code != 0 {
		t.Errorf("RunCheck = %d, want 0", code)
	}
}

// A media folder is reported when a file is there that nothing references, and
// when a referenced image is heavy enough to hurt a phone.
func TestMediaWarnings(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	media := filepath.Join(dir, "media")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "orphan.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(mediaWarnings(cfg, media), "\n")
	if !strings.Contains(warnings, "orphan.png") {
		t.Errorf("an unreferenced media file was not reported:\n%s", warnings)
	}
}

// The exit code is the contract: anything that would not boot has to be a 1,
// whether the YAML is invalid or the directory is not there at all.
func TestRunCheckRefusals(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
	}{
		{"a service with no url", map[string]string{"services.yaml": "- {id: pad, name: Pad}\n"}},
		{"a site that is not a mapping", map[string]string{
			"site.yaml":     "- just a list\n",
			"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n"}},
		{"no services file at all", map[string]string{"site.yaml": "locales: [en]\n"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if code := RunCheck(testutil.WriteFiles(t, c.files)); code != 1 {
				t.Errorf("RunCheck = %d, want 1", code)
			}
		})
	}
	if code := RunCheck(filepath.Join(t.TempDir(), "absent")); code != 1 {
		t.Errorf("RunCheck on a missing directory = %d, want 1", code)
	}
}

// site.url is what builds the canonical, og:url and hreflang links. Without
// it they are simply absent, the page looks perfectly fine, and a multilingual
// site reads to a crawler as several duplicates of each other. Nothing else
// tells the operator, which is the whole reason this warning exists.
func TestCheckWarnsAboutMissingSiteURL(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [fr, en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{"no url", "canonical", "hreflang"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("missing url warning has no %q:\n%s", want, warnings)
		}
	}
}

// An operator who asked search engines to stay away has already answered the
// question. Warning them anyway is how a check earns being ignored.
func TestCheckStaysQuietAboutURLWhenNoindex(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nindex: false\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(checkWarnings(cfg, dir), "\n"); strings.Contains(w, "canonical") {
		t.Errorf("noindex site was nagged about canonical links:\n%s", w)
	}
}

// A vector logo looks like a logo, renders fine in the header, and produces no
// og:image at all: most platforms ignore svg for a preview card. The page
// gives no sign of it, so the check has to.
func TestCheckWarnsAboutAVectorLogo(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlogo: /assets/logo.svg\nlocales: [en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(w, "not a raster") || !strings.Contains(w, "logo.svg") {
		t.Errorf("a vector logo went unmentioned:\n%s", w)
	}
}

// A bare filename in logo, favicon or a link url is passed through untouched,
// so the browser resolves it against whatever page it is on: logo.png is
// /en/logo.png from the home page and /logo.png from the manifest, and 404s
// from both. Nothing in a running site says so, the header simply shows a
// broken image, which is why -check has to.
func TestCheckWarnsAboutRefsThatResolveNowhere(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [en]\n" +
			"logo: logo.png\nfavicon: favicon.ico\n" +
			"links:\n  - {label: Wiki, url: wiki.example.org}\n" +
			"footer:\n  - {label: Contact, url: mailto:hello@example.org}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")

	for _, want := range []string{
		`logo "logo.png" resolves nowhere`,
		`favicon "favicon.ico" resolves nowhere`,
		`links url "wiki.example.org" resolves nowhere`,
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	// A scheme is a scheme: mailto: and tel: are the two an operator writes in
	// a footer, and flagging them is how a check earns being ignored.
	if strings.Contains(warnings, "mailto:hello@example.org") {
		t.Errorf("a mailto: link was flagged as unresolvable:\n%s", warnings)
	}
}

// Two categories differing only in case render as two sections carrying the
// same name, one of them nearly empty. It reads as a rendering bug and it is a
// typo in a services.yaml, so the check names both spellings.
func TestCheckWarnsAboutCategoriesDifferingOnlyByCase(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlogo: /assets/logo.png\nlocales: [en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, category: Documents}\n" +
			"- {id: b, url: https://b.example.org, name: B, category: documents}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{"differ only by case", `"Documents"`, `"documents"`} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
}

// A file in media/ can be written two ways, and the docs show both: the bare
// name and the /media/… path it is served at. Recording only the first made
// -check announce a file as unreferenced while it sat on the page, which is
// worse than silence: it invites deleting an image that is in use.
func TestCheckCountsBothSpellingsOfAMediaReference(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlogo: /assets/logo.png\nlocales: [en]\n" +
			"about: \"Layout: ![plan](/media/plan.png)\"\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, images: [/media/shot.png]}\n" +
			"- {id: doc, url: https://doc.example.org, name: Doc, images: [manual.png]}\n",
	})
	media := filepath.Join(dir, "media")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shot.png", "manual.png", "plan.png", "orphan.png"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(mediaWarnings(cfg, media), "\n")

	for _, used := range []string{"shot.png", "manual.png", "plan.png"} {
		if strings.Contains(warnings, used) {
			t.Errorf("%s is on a page and was reported unreferenced:\n%s", used, warnings)
		}
	}
	// And the warning still fires for a file nothing points at, or the test
	// above would pass just as well with the check switched off.
	if !strings.Contains(warnings, "orphan.png") {
		t.Errorf("a genuinely unreferenced file went unreported:\n%s", warnings)
	}
}

// Without a url there is no og:image to miss, and the url warning already
// covers it. Saying both would be saying the same thing twice.
func TestCheckDoesNotStackTheLogoWarningOnTheURLOne(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(checkWarnings(cfg, dir), "\n"); strings.Contains(w, "og:image") {
		t.Errorf("a config with no url was also nagged about og:image:\n%s", w)
	}
}

// Four settings that are accepted, then do nothing, with no error and no hint
// from the page. Each is written the way an operator would write it.
func TestCheckNamesSettingsThatDoNothing(t *testing.T) {
	for _, c := range []struct {
		name, site string
		want       string
	}{
		{"status keys with nothing to poll",
			"locales: [en]\nindex: false\nstatus: {page: https://s.example.org, interval: 30s}\n",
			"without status.gatus"},
		{"a misspelled strings override",
			"locales: [en]\nindex: false\nstrings: {card.mode: Read on}\n",
			`strings key "card.mode" is not one cairn uses`},
		{"an icons list that drops the install pair",
			"locales: [en]\nindex: false\nicons: [{src: /assets/a.png, sizes: 180x180}]\n",
			"stop offering to install"},
		{"an icon on a footer entry",
			"locales: [en]\nindex: false\nfooter: [{label: C, url: https://c.example.org, icon: mail}]\n",
			"only header links render one"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := testutil.WriteFiles(t, map[string]string{
				"site.yaml":     c.site,
				"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
			})
			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			w := strings.Join(checkWarnings(cfg, dir), "\n")
			if !strings.Contains(w, c.want) {
				t.Errorf("no warning carrying %q:\n%s", c.want, w)
			}
		})
	}
}

// The same four keys, used the way they are meant to be, must stay silent: a
// warning that fires on a correct config is how the whole set gets ignored.
func TestCheckStaysQuietWhenThoseSettingsWork(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nindex: false\n" +
			"status: {gatus: https://s.example.org, page: https://s.example.org, interval: 30s, linked: false}\n" +
			"strings: {card.more: Read on}\n" +
			"icons: [{src: /assets/a.png, sizes: 192x192}, {src: /assets/b.png, sizes: 512x512}]\n" +
			"footer: [{label: C, url: https://c.example.org}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(inertSettings(cfg), "\n"); w != "" {
		t.Errorf("a correct config was warned about:\n%s", w)
	}
}
