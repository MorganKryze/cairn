package render

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// update rewrites the golden files instead of comparing against them:
//
//	go test ./src/internal/render -run TestAGatusSiteRendersWhatItAlwaysDid -update
//
// Rewriting is the point at which a diff has to be read line by line and
// justified in the pull request. That is the whole ceremony this file exists
// to impose, so it is deliberately not automatic.
var update = flag.Bool("update", false, "rewrite the golden files")

// goldenSite is one site exercising everything a Gatus deployment touches on
// the way to bytes: two locales so the fallback marking is in play, a service
// with a detail page, a service Gatus does not monitor, an uncategorised one,
// a host flag, a footer, a logo, markdown prose, and a status page the pills
// link to. Anything that changes the shape of a page changes this file.
const goldenSite = `title: Golden
tagline:
  en: Everything that must not move.
  fr: Tout ce qui ne doit pas bouger.
locales: [en, fr]
url: https://golden.example.org
theme:
  accent: "#247b7b"
about:
  en: |
    A welcome note, with *emphasis* and a [link](https://example.org).
show_version: true
status:
  gatus: https://status.example.org
  interval: 30s
footer:
  - label: { en: Status, fr: Statut }
    url: https://status.example.org
`

const goldenServices = `- id: pdf
  url: https://pdf.example.org
  category: documents
  name: { en: PDF toolbox, fr: Boîte à outils PDF }
  desc: { en: Merge and split PDFs., fr: Fusionner et découper vos PDF. }
  selfhosted: true
  tags: [pdf, convert]
  details:
    en: |
      ## What it does

      1. Drop a file
      2. Pick an action

      | Format | In | Out |
      | --- | --- | --- |
      | pdf | yes | yes |
- id: pad
  url: https://pad.example.org
  category: documents
  name: Shared notepad
  desc: Write together.
- id: wiki
  url: /wiki
  name: Wiki
`

// A Gatus site is what every cairn deployment with status pills runs today.
// The multi-source work adds providers beside it; this pins that "beside" is
// what it stays. A byte that moves here is a byte that moved on somebody's
// running site, and it has to be argued for rather than noticed later.
func TestAGatusSiteRendersWhatItAlwaysDid(t *testing.T) {
	prev := Version
	Version = "1.14.0" // pinned: the footer prints it, and the build stamps it
	t.Cleanup(func() { Version = prev })

	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     goldenSite,
		"services.yaml": goldenServices,
	}))
	if err != nil {
		t.Fatal(err)
	}
	// Gatus has answered: pdf up, pad down, and it says nothing about wiki,
	// which is the third of the three states a card can be in.
	m, err := BuildModel(cfg, map[string]status.State{
		"pdf": {Level: status.LevelUp, Key: "documents_pdf"},
		"pad": {Level: status.LevelDown, Key: "documents_pad"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, page := range []string{"en", "fr", "en/pdf", "fr/pdf"} {
		name := filepath.Join("testdata", "golden", filepath.FromSlash(page)+".html")
		compare(t, name, m.Pages[page].HTML)
	}
	compare(t, filepath.Join("testdata", "golden", "csp.txt"), []byte(BuildCSP(cfg)))

	// And the pill hrefs, called out separately: they are the one thing the
	// provider work is most likely to move, and a diff in a 30 KB page is easy
	// to skim past.
	for _, want := range []string{
		`href="https://status.example.org/endpoints/documents_pdf"`,
		`href="https://status.example.org/endpoints/documents_pad"`,
	} {
		if !contains(m.Pages["en"].HTML, want) {
			t.Errorf("the home page no longer carries %s", want)
		}
	}
}

// blurDigests replaces the content digest in an asset url with a fixed word,
// so the golden files pin the markup rather than the bytes of the stylesheet.
//
// Without it every edit to style.css or a script rewrites four golden files,
// because the url they are linked under moves with their contents, which is
// the whole point of the digest and no business of a markup net. What the
// digest actually is has its own tests: it is computed from the file in
// TestTheDigestIsOfTheFilesOwnBytes, and no page may link an asset without one
// in TestNoPageLinksAnUnstampedAsset. Blurring it here loses nothing those two
// do not already hold, and stops the net crying wolf.
var digestInURL = regexp.MustCompile(`(/static/[A-Za-z0-9._/-]+?)\.[0-9a-f]{8}\.`)

func blurDigests(b []byte) []byte {
	return digestInURL.ReplaceAll(b, []byte("$1.DIGEST."))
}

func compare(t *testing.T, name string, got []byte) {
	t.Helper()
	got = blurDigests(got)
	if *update {
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, got, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s (%d bytes)", name, len(got))
		return
	}
	want, err := os.ReadFile(name) // #nosec G304 -- a path this test built
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(got) == string(want) {
		return
	}
	t.Errorf("%s changed: %d bytes now, %d before.\n%s", name, len(got), len(want), firstDiff(want, got))
}

// firstDiff points at the byte that moved, with a little either side. A
// unified diff of a whole page buries the one line that matters.
func firstDiff(want, got []byte) string {
	i := 0
	for i < len(want) && i < len(got) && want[i] == got[i] {
		i++
	}
	from := max(0, i-90)
	return "  before: …" + string(want[from:min(len(want), i+90)]) +
		"…\n  after:  …" + string(got[from:min(len(got), i+90)]) + "…"
}

func contains(hay []byte, needle string) bool {
	return len(needle) <= len(hay) && indexOf(string(hay), needle) >= 0
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
