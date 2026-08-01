package check

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// withAssets points config.AssetsPath at a directory of the test's own and
// fills it, returning the path so a test can assert the warning names it.
//
// Every /assets check is skipped when the mount is absent, and the default
// /assets exists in the container and on no developer's machine: a test that
// left the global alone would assert one thing here and the opposite there.
func withAssets(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := config.AssetsPath
	config.AssetsPath = dir
	t.Cleanup(func() { config.AssetsPath = old })
	return dir
}

// withoutAssets points config.AssetsPath at a directory that is not there,
// which is what -check on a laptop actually has.
func withoutAssets(t *testing.T) {
	t.Helper()
	old := config.AssetsPath
	config.AssetsPath = filepath.Join(t.TempDir(), "never-mounted")
	t.Cleanup(func() { config.AssetsPath = old })
}

// pngOf encodes a blank raster of a given size. The icon check compares a
// declared size against a measured one, so the file has to really be that
// size or the test proves nothing.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

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
	withAssets(t, map[string][]byte{"logo.png": pngOf(t, 512, 512)})
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

// The mirror of the test above. Load opens every images: entry and refuses to
// boot on a missing one, which left markdown as the one way to ship a picture
// nothing serves: it reads exactly like the images: form in the docs, and no
// check ever opened it.
func TestCheckWarnsAboutAMarkdownImageThatIsNotThere(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nabout: \"See ![plan](plan.png)\"\n" +
			"pages: [{id: legal, title: Legal, body: \"![seal](/media/seal.png)\"}]\n" +
			"tagline: x\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, details: \"![shot](sub/shot.png)\"}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")

	// Every spelling, including the reference form a regexp of our own could
	// never see: -check asks the parser that renders the page.
	for _, want := range []string{
		`site about shows "plan.png"`,
		`page "legal" body shows "seal.png"`,
		`service "a" details shows "sub/shot.png"`,
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings never mention %s:\n%s", want, warnings)
		}
	}
}

// The same check must stay silent about everything it cannot open, or it
// reports a broken image for every remote screenshot on the site.
func TestCheckStaysQuietAboutMarkdownImagesItCannotStat(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nabout: \"![a](https://cdn.example.org/a.png) " +
			"![b](/assets/b.png) ![c](data:image/gif;base64,R0lGOD) ![d](../../etc/passwd)\"\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range checkWarnings(cfg, dir) {
		if strings.Contains(w, "renders a broken image") {
			t.Errorf("a reference cairn cannot stat was reported missing:\n%s", w)
		}
	}
}

// status.insecure is a decision rather than a mistake, and this warning is the
// only trace it leaves once the startup log has scrolled away. It has to fire
// every run, and only when there is actually a gatus to not-verify.
func TestCheckKeepsSayingInsecureIsOn(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nindex: false\nstatus: {gatus: \"https://status.internal\", insecure: true}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{"status.insecure is on", "https://status.internal", "CA bundle"} {
		if !strings.Contains(w, want) {
			t.Errorf("warnings never mention %q:\n%s", want, w)
		}
	}
}

// A bundle fetched over http lands in the same place status.insecure does:
// whatever is on the path to that address decides what cairn trusts. The key
// is allowed, because an internal PKI often has nowhere better to serve it
// from, so the warning is what pays for it. https and a mounted file are the
// shapes that need no warning, and one of them has to stay quiet or the
// message means nothing.
func TestCheckNamesAnUnverifiedCABundle(t *testing.T) {
	withoutAssets(t)
	loud := checkFor(t, "status: {gatus: \"https://status.internal\", ca: \"http://pki.internal/ca.crt\"}")
	for _, want := range []string{"status.ca", "http://pki.internal/ca.crt", "over http"} {
		if !strings.Contains(loud, want) {
			t.Errorf("warnings never mention %q:\n%s", want, loud)
		}
	}
	quiet := checkFor(t, "status: {gatus: \"https://status.internal\", ca: \"https://pki.internal/ca.crt\"}")
	if strings.Contains(quiet, "status.ca") {
		t.Errorf("a bundle fetched over https was warned about anyway:\n%s", quiet)
	}
}

// Both set is not a stricter site, it is the loose one: verification is off, so
// the bundle is never consulted. An operator who wrote both meant the bundle.
func TestCheckSaysTheBundleIsDeadUnderInsecure(t *testing.T) {
	withoutAssets(t)
	w := checkFor(t, "status: {gatus: \"https://status.internal\", insecure: true, ca: \"/assets/ca.crt\"}")
	if !strings.Contains(w, "status.ca") || !strings.Contains(w, "status.insecure") {
		t.Errorf("warnings do not say the bundle is dead under insecure:\n%s", w)
	}
}

// Every companion key needs an address to act on, and this one is no different.
func TestCheckReportsCAWithoutGatus(t *testing.T) {
	withoutAssets(t)
	w := checkFor(t, "status: {ca: \"https://pki.internal/ca.crt\"}")
	if !strings.Contains(w, "status.ca") || !strings.Contains(w, "nothing polls") {
		t.Errorf("status.ca without a gatus went unreported:\n%s", w)
	}
}

// A provider is a companion key like the others: it says what answers at the
// address, and with no address it answers nothing. Naming a monitor and never
// saying where it lives is a config that boots, validates and draws no pill.
func TestCheckReportsAProviderWithNoAddress(t *testing.T) {
	withoutAssets(t)
	w := checkFor(t, "status: {provider: gatus}")
	if !strings.Contains(w, "status.provider") || !strings.Contains(w, "nothing polls") {
		t.Errorf("a provider with nothing to poll went unreported:\n%s", w)
	}
	// The message used to name one key because there was one; an operator who
	// wrote status.url has to be told that is the other half of the answer.
	if !strings.Contains(w, "status.url") {
		t.Errorf("the message names only one of the two ways to give an address:\n%s", w)
	}
}

// A bundle named but not mounted is the loudest failure of the three: cairn
// has nothing to verify against, so every poll fails and the pills never come
// back. Only -check can see it before the site is running.
func TestCheckNamesAMissingCABundle(t *testing.T) {
	assets := t.TempDir()
	config.AssetsPath = assets
	t.Cleanup(func() { config.AssetsPath = "/assets" })

	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nindex: false\nstatus: {gatus: \"https://status.internal\", ca: \"/assets/ca.crt\"}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(w, "status.ca") || !strings.Contains(w, "grey") {
		t.Errorf("a bundle that is not on disk went unreported:\n%s", w)
	}

	// And says nothing once the file is there.
	if err := os.WriteFile(filepath.Join(assets, "ca.crt"), []byte("-----BEGIN CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(checkWarnings(cfg, dir), "\n"); strings.Contains(w, "not in the assets directory") {
		t.Errorf("a mounted bundle was still reported missing:\n%s", w)
	}
}

// checkFor runs the warnings over a one-service site carrying the given status
// block, and joins them.
func checkFor(t *testing.T, statusBlock string) string {
	t.Helper()
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nindex: false\n" + statusBlock + "\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(checkWarnings(cfg, dir), "\n")
}

// And stays quiet when it is off, which is the shape every other site has.
func TestCheckSaysNothingWhenInsecureIsOff(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nindex: false\nstatus: {gatus: \"https://status.internal\"}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range checkWarnings(cfg, dir) {
		if strings.Contains(w, "insecure") {
			t.Errorf("a verifying site was warned about verification:\n%s", w)
		}
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
	// The logo really sits in the assets directory here, so this also pins
	// that a reference which does resolve draws no warning.
	withAssets(t, map[string][]byte{"logo.png": pngOf(t, 512, 512)})
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

// og:image is what Slack, Mastodon and Signal paint when someone pastes the
// link, and not one of them is a search engine. index: false answers the
// crawler question and says nothing about the preview, and a portal kept out
// of search results is exactly the one that travels by pasted link.
func TestCheckWarnsAboutOGImageEvenWhenNoindex(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlocales: [en]\nindex: false\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(w, "og:image") {
		t.Errorf("a noindex site was left with no link preview and no warning:\n%s", w)
	}
	// The canonical family keeps its gate. Without this the test would pass
	// just as well by removing the wrong one.
	if strings.Contains(w, "canonical") {
		t.Errorf("noindex no longer silences the canonical warning:\n%s", w)
	}
}

// A logo, favicon, manifest icon or link icon under /assets is passed to the
// browser as written and never confirmed. Load already refuses to boot over a
// mistyped screenshot in media/, so the same typo one field over produces a
// broken image on every page and not one word anywhere.
func TestCheckWarnsAboutAssetsThatAreNotThere(t *testing.T) {
	assets := withAssets(t, map[string][]byte{"logo.png": pngOf(t, 512, 512)})
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [en]\n" +
			"logo: /assets/logo.png\nfavicon: /assets/favicon.png\n" +
			"icons: [{src: /assets/icon-192.png, sizes: 192x192}, {src: /assets/logo.png, sizes: 512x512}]\n" +
			"links: [{label: Wiki, url: https://wiki.example.org, icon: /assets/wiki.svg}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")

	for _, want := range []string{
		`favicon "/assets/favicon.png" is not in the assets directory`,
		`icons entry 1 src "/assets/icon-192.png" is not in the assets directory`,
		`links entry "https://wiki.example.org" icon "/assets/wiki.svg" is not in the assets directory`,
		// The message has to name the directory cairn looked in: the usual
		// cause is a forgotten -assets, and the reference itself looks right.
		filepath.Join(assets, "favicon.png"),
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	// The two references that do resolve stay unmentioned.
	if strings.Contains(warnings, `logo "/assets/logo.png" is not in`) ||
		strings.Contains(warnings, `icons entry 2`) {
		t.Errorf("a file that is really there was reported missing:\n%s", warnings)
	}
}

// Everything that is not a file in the mount is somebody else's to resolve: a
// remote URL would need a request cairn does not make, another route is not
// this directory's business, and a bare name already has its own warning. A
// check that flagged them would fire on every correct config there is.
func TestCheckStaysQuietAboutRefsItCannotStat(t *testing.T) {
	withAssets(t, map[string][]byte{"logo.png": pngOf(t, 512, 512)})
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [en]\n" +
			"logo: https://cdn.example.org/logo.png\nfavicon: /static/favicon.ico\n" +
			"icons: [{src: /assets/logo.png, sizes: 512x512}, {src: https://cdn.example.org/i.png, sizes: 192x192}]\n" +
			"links: [{label: Wiki, url: https://wiki.example.org, icon: /media/wiki.png}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(checkWarnings(cfg, dir), "\n"); strings.Contains(w, "assets directory") {
		t.Errorf("a reference cairn cannot stat was reported missing:\n%s", w)
	}
}

// The default -assets is /assets, which exists in the container and on nobody
// else's machine. Statting it anyway would make -check on a laptop print a
// warning for every asset the site has, all of them wrong, and a page of
// false warnings is worse than the silence being fixed: it buries the real
// ones and teaches the operator to skip the output.
func TestCheckSaysNothingAboutAssetsWithNothingMounted(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [en]\n" +
			"logo: /assets/logo.png\nfavicon: /assets/favicon.png\n" +
			"icons: [{src: /assets/i-192.png, sizes: 192x192}, {src: /assets/i-512.png, sizes: 512x512}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := checkWarnings(cfg, dir); len(w) != 0 {
		t.Errorf("with no assets directory to look in, -check invented warnings: %v", w)
	}
}

// cairn cannot resize an image, so the operator states each icon's size and
// the manifest publishes it as fact. A wrong number shows up nowhere: the
// manifest validates, the file loads, and the phone simply picks it for a
// slot it does not fill.
func TestCheckMeasuresIconsAgainstTheirDeclaredSizes(t *testing.T) {
	withAssets(t, map[string][]byte{
		"small.png": pngOf(t, 180, 180),
		"i-192.png": pngOf(t, 192, 192),
		"i-512.png": pngOf(t, 512, 512),
	})
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nindex: false\n" +
			"icons: [{src: /assets/small.png, sizes: 512x512}, " +
			"{src: /assets/i-192.png, sizes: 192x192}, {src: /assets/i-512.png, sizes: 512x512}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(warnings, `icons entry 1 declares sizes "512x512" but /assets/small.png measures 180x180`) {
		t.Errorf("an icon that is not the size it claims went unmentioned:\n%s", warnings)
	}
	// The two honest entries are not mentioned, or the warning would fire on
	// every icons list ever written.
	for _, quiet := range []string{"entry 2 declares", "entry 3 declares"} {
		if strings.Contains(warnings, quiet) {
			t.Errorf("an icon that is exactly its declared size was flagged:\n%s", warnings)
		}
	}
}

// Three shapes that are honest and must stay silent: "any" on a scalable
// file, which is a claim and not a measurement; a multi-size entry where one
// of the sizes is the file's, which is how an ico is written; and a file
// nothing can decode, which is unknown rather than wrong.
func TestCheckDoesNotSecondGuessIconsItCannotMeasure(t *testing.T) {
	withAssets(t, map[string][]byte{
		"mark.svg":  []byte(`<svg viewBox="0 0 9 9"/>`),
		"multi.png": pngOf(t, 32, 32),
		"i-192.png": pngOf(t, 192, 192),
		"i-512.png": pngOf(t, 512, 512),
	})
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nindex: false\n" +
			"icons: [{src: /assets/mark.svg, sizes: any}, {src: /assets/multi.png, sizes: 16x16 32x32}, " +
			"{src: /assets/i-192.png, sizes: 192x192}, {src: /assets/i-512.png, sizes: 512x512}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(checkWarnings(cfg, dir), "\n"); strings.Contains(w, "declares sizes") {
		t.Errorf("an icon that cairn cannot contradict was flagged anyway:\n%s", w)
	}
}

// Four fields that fill one language's page with another's words, including
// the browser tab. The check covered the body text and missed the labels.
func TestCheckNamesHalfTranslatedTitlesLabelsAndCategories(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nindex: false\n" +
			"title: {fr: Outils libres}\n" +
			"links: [{label: {fr: Annuaire}, url: https://wiki.example.org}]\n" +
			"footer: [{label: {fr: Statut}, url: https://status.example.org}]\n",
		"categories.yaml": "- {id: docs, name: {fr: Documents}}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, category: docs}\n" +
			"- {id: b, url: https://b.example.org, name: B}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{
		"site title has no en",
		`links entry "https://wiki.example.org" label has no en`,
		`footer entry "https://status.example.org" label has no en`,
		`category "docs" name has no en`,
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	// Service b carries no category, so it lands in "other", which has no
	// name at all. An absent name is derived from the id in every locale,
	// which is documented behaviour and not a missing translation.
	if strings.Contains(warnings, `category "other"`) {
		t.Errorf("a category with no name was called half-translated:\n%s", warnings)
	}
}

// Str resolves a strings: override per locale and falls through to the
// built-in table on a miss, so an override that names some locales gives the
// operator's wording to those pages and cairn's to the rest. It is the one
// case where someone asked for different words and got them on half the site.
func TestCheckWarnsAboutAStringsOverrideThatSkipsALocale(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nindex: false\n" +
			"strings: {nav.menu: {fr: Sommaire}, card.more: {fr: Lire la suite, en: Read on}, " +
			"nav.tocs: {fr: Rubriques}}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(warnings, `strings key "nav.menu" has no en`) {
		t.Errorf("a half-covered override went unmentioned:\n%s", warnings)
	}
	// An override that covers every locale is exactly right.
	if strings.Contains(warnings, `"card.more" has no`) {
		t.Errorf("a fully covered override was flagged:\n%s", warnings)
	}
	// nav.tocs is a typo for nav.toc. inertSettings already says the whole
	// override does nothing, and which locales it covers is beside the point
	// then: two warnings over one line is how a check gets skimmed.
	if strings.Contains(warnings, `"nav.tocs" has no`) {
		t.Errorf("a key that does nothing was also nagged about locales:\n%s", warnings)
	}
	if !strings.Contains(warnings, `strings key "nav.tocs" is not one cairn uses`) {
		t.Errorf("the typo lost its own warning:\n%s", warnings)
	}
}

// cairn dresses its interface in seven languages. Any other locale renders
// with the right lang attribute and an entirely English menu, and nobody is
// told: the pages build, they validate, and only a reader notices.
func TestCheckWarnsAboutALocaleWithNoInterface(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en, pl]\nindex: false\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{`locale "pl" has no built-in interface`, "strings:"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	if strings.Contains(warnings, `locale "en"`) {
		t.Errorf("a language cairn ships was reported as unsupported:\n%s", warnings)
	}
}

// Str looks the base language up after the full tag, so pt-BR finds the pt
// table and those pages are dressed in Portuguese. Warning about every
// regional variant of a language cairn does ship is the opposite of helpful.
func TestCheckAcceptsARegionalVariantOfABuiltinLocale(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [pt-BR, fr-CA]\nindex: false\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(checkWarnings(cfg, dir), "\n"); strings.Contains(w, "built-in interface") {
		t.Errorf("a regional variant of a shipped language was flagged:\n%s", w)
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

// The spelling a regexp cannot see. `![cap][ref]` with its definition further
// down rendered a broken image while -check printed ok, because the old
// pattern knew only `![…](…)`. The check reads the AST now, so both forms and
// the shortcut form all arrive.
func TestCheckSeesReferenceStyleImages(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\n" +
			"about: \"![cap][ref]\\n\\n[ref]: from-ref.png\"\n" +
			"pages: [{id: legal, title: Legal, body: \"![short]\\n\\n[short]: shortcut.png\"}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{`site about shows "from-ref.png"`, `page "legal" body shows "shortcut.png"`} {
		if !strings.Contains(w, want) {
			t.Errorf("warnings never mention %s:\n%s", want, w)
		}
	}
}

// cairn demotes a single # rather than emitting a second <h1>, and the warning
// is what stops that being a puzzle: without it, # and ## produce identical
// output and nothing explains why.
func TestCheckExplainsTheDemotedHeading(t *testing.T) {
	withoutAssets(t)
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nindex: false\nabout: \"# My notes\\n\\nText.\"\n" +
			"pages: [{id: legal, title: Legal, body: \"## Fine\\n\\nText.\"}]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(checkWarnings(cfg, dir), "\n")
	if !strings.Contains(w, "site about opens a heading with a single #") {
		t.Errorf("the # was not explained:\n%s", w)
	}
	// The page opens at ##, which is the shape being recommended: it must not
	// be nagged about, or the warning teaches people to ignore the output.
	if strings.Contains(w, `page "legal" body opens a heading`) {
		t.Errorf("a page that already starts at ## was warned about:\n%s", w)
	}
}
