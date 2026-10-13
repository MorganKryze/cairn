package render

import (
	"bytes"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// shotPages builds a site whose pad service shows a local 4x1 PNG, a size
// cairn reads, and an image under /assets/, whose size it does not.
func shotPages(t *testing.T, site, images string) *Model {
	t.Helper()
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     site,
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, images: " + images + "}\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 1))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "shot.png"), buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

var figureRe = regexp.MustCompile(`(?s)<figure class="shot">.*?</figure>`)

func figures(t *testing.T, m *Model, page string) []string {
	t.Helper()
	got := figureRe.FindAllString(string(m.Pages[page].HTML), -1)
	if len(got) == 0 {
		t.Fatalf("page %s shows no screenshot", page)
	}
	return got
}

var fullLinkRe = regexp.MustCompile(`<a class="shot-full"[^>]*>`)

// The strip is sized from the image's own ratio, which only the width and
// height attributes give it: a contained image with no ratio collapses. So it
// is only drawn where cairn knows both, and it is never given a tabindex, since
// a browser makes a scrolling region a stop only while it scrolls.
func TestAScreenshotScrollsOnlyWhenItsSizeIsKnown(t *testing.T) {
	m := shotPages(t, "locales: [en]\n", "[shot.png, /assets/logo.png]")
	shots := figures(t, m, "en/pad")
	if len(shots) != 2 {
		t.Fatalf("want two screenshots, got %d", len(shots))
	}
	const strip = `<figure class="shot"><div class="shot-scroll" role="region" aria-label="Screenshot, scroll sideways">` +
		`<img src="/media/shot.png" alt="" loading="lazy" width="4" height="1"></div>`
	if !strings.HasPrefix(shots[0], strip) {
		t.Errorf("a screenshot of known size is not a named strip:\n%s", shots[0])
	}
	if strings.Contains(shots[0], "tabindex") {
		t.Errorf("the strip sets its own tabindex:\n%s", shots[0])
	}
	if !strings.HasPrefix(shots[1], `<figure class="shot"><img src="/assets/logo.png" alt="" loading="lazy">`) ||
		strings.Contains(shots[1], "shot-scroll") {
		t.Errorf("a screenshot of unknown size is not the bare image:\n%s", shots[1])
	}
}

// The link is the way to the whole picture at every width: the file itself, in
// the same tab, never through the leave dialog, in the page's own words, on a
// site that opens services in a new tab behind that dialog.
func TestTheFullScreenshotLinkOpensTheImageItself(t *testing.T) {
	m := shotPages(t, "locales: [fr, en]\nservice_links: {new_tab: true, confirm: all}\n", "[shot.png, /assets/logo.png]")
	for _, shot := range figures(t, m, "fr/pad") {
		src := regexp.MustCompile(`<img src="([^"]*)"`).FindStringSubmatch(shot)
		link := fullLinkRe.FindString(shot)
		if src == nil || link == "" {
			t.Errorf("a screenshot has no image or no full link:\n%s", shot)
			continue
		}
		if want := `href="` + src[1] + `"`; !strings.Contains(link, want) {
			t.Errorf("the link %s does not lead to the image's own %s", link, want)
		}
		for _, attr := range []string{"target=", "rel=", "data-leave", "aria-haspopup"} {
			if strings.Contains(link, attr) {
				t.Errorf("the link carries %s: %s", attr, link)
			}
		}
		if !strings.Contains(shot, "Voir la capture entière</a>") {
			t.Errorf("the link does not read the page's own words:\n%s", shot)
		}
	}
}

// A caption that falls back to another language is marked as such, and only
// the caption: the link under it is a cairn string in the page's language.
func TestACaptionSitsAboveTheLinkAndMarksOnlyItself(t *testing.T) {
	m := shotPages(t, "locales: [fr, en]\n", "[{src: shot.png, caption: {en: The editor.}}]")
	shot := figures(t, m, "fr/pad")[0]
	caption := strings.Index(shot, `<figcaption lang="en">The editor.</figcaption>`)
	link := fullLinkRe.FindStringIndex(shot)
	switch {
	case caption < 0:
		t.Errorf("the fallback caption lost its language mark:\n%s", shot)
	case link == nil:
		t.Errorf("no full link:\n%s", shot)
	case link[0] < caption:
		t.Errorf("the link comes before the caption:\n%s", shot)
	case strings.Contains(shot[link[0]:], "lang="):
		t.Errorf("the link carries a language mark:\n%s", shot)
	}
}

// Both words are ordinary strings an operator can reword, and nothing under
// the embedded templates and assets spells them out in English instead.
func TestTheScreenshotWordsComeFromTheTable(t *testing.T) {
	m := shotPages(t, "locales: [en]\n"+
		"strings: {shot.scroll: {en: \"Swipe the picture\"}, shot.full: {en: \"Whole picture\"}}\n",
		"[shot.png]")
	shot := figures(t, m, "en/pad")[0]
	if !strings.Contains(shot, `aria-label="Swipe the picture"`) {
		t.Errorf("the strip does not read shot.scroll:\n%s", shot)
	}
	if !strings.Contains(shot, "Whole picture</a>") {
		t.Errorf("the link does not read shot.full:\n%s", shot)
	}

	english := []string{"About this site", "Screenshot, scroll sideways", "See the full screenshot"}
	err := fs.WalkDir(Embedded, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := Embedded.ReadFile(p)
		if err != nil {
			return err
		}
		for _, s := range english {
			if bytes.Contains(b, []byte(s)) {
				t.Errorf("%s spells out %q instead of reading the strings table", p, s)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Under a sub-path the link takes the prefix exactly as the image does, or it
// leads to a file the server does not have there.
func TestTheFullScreenshotLinkKeepsTheBasePath(t *testing.T) {
	old := BasePath
	BasePath = "/tools"
	defer func() { BasePath = old }()
	m := shotPages(t, "locales: [en]\n", "[shot.png]")
	shot := figures(t, m, "en/pad")[0]
	if !strings.Contains(shot, `<img src="/tools/media/shot.png"`) {
		t.Fatalf("the image does not carry the base path:\n%s", shot)
	}
	if !strings.Contains(shot, `<a class="shot-full" href="/tools/media/shot.png">`) {
		t.Errorf("the link does not carry the base path the image does:\n%s", shot)
	}
}
