package render

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// Written after reviewing the digest work rather than while writing it, which
// is why these look for the gaps the first pass left rather than for the
// behaviour it set out to build. Each one holds a way the scheme can be
// half-undone later without anything going red.

// A name in a template that cairn does not ship comes back as the plain path,
// which is a 404 the browser meets silently: the page renders unstyled and
// every test that reads markup still passes. So the names are checked against
// what is actually embedded.
func TestEveryAssetATemplateAsksForIsShipped(t *testing.T) {
	asks := regexp.MustCompile(`asset "([^"]+)"`)
	found := 0
	err := fs.WalkDir(Embedded, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := Embedded.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range asks.FindAllStringSubmatch(string(b), -1) {
			found++
			if _, err := Embedded.ReadFile("assets/" + m[1]); err != nil {
				t.Errorf("%s asks for %q, which is not in assets/", p, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("no template asks for an asset, so this checked nothing")
	}
}

// The regression this whole change exists to prevent, stated as a rule a page
// has to keep obeying: nothing cairn serves may be linked under a name that
// does not move when its contents do. Adding a script with a hand-written
// /static/ path would reintroduce the stale-after-upgrade bug for that one
// file, and no other test here would notice.
func TestNoPageLinksAnUnstampedAsset(t *testing.T) {
	stamped := regexp.MustCompile(`^/static/(.+)\.[0-9a-f]{8}(\.[a-z0-9]+)?$`)
	any := regexp.MustCompile(`/static/[A-Za-z0-9._/-]+`)

	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "title: Test\nlocales: [en]\n",
		"services.yaml": "- {id: pad, url: 'https://pad.example.org', name: Pad, desc: D., details: More.}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, map[string]status.State{})
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for name, p := range m.Pages {
		for _, u := range any.FindAllString(string(p.HTML), -1) {
			seen++
			// The font is the documented exception: style.css reaches it by a
			// relative url() that nothing out here can rewrite. See asseturl.go.
			if strings.HasPrefix(u, "/static/fonts/") {
				continue
			}
			if !stamped.MatchString(u) {
				t.Errorf("page %s links %s, which carries no digest", name, u)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no page linked anything under /static/, so this checked nothing")
	}
}

// The manifest and the apple-touch-icon are built in Go rather than written in
// a template, which is exactly how they were missed: they never met the asset
// function and shipped unstamped while everything around them moved.
func TestTheIconsBuiltInGoAreStampedToo(t *testing.T) {
	for _, got := range []string{
		AppURL(defaultTouchIcon),
		AppURL("/static/icon-192.png"),
		AppURL("/static/icon-512.png"),
	} {
		if !regexp.MustCompile(`\.[0-9a-f]{8}\.png$`).MatchString(got) {
			t.Errorf("%s carries no digest", got)
		}
	}
}

// And the other half of that: cairn stamps only what it ships. An operator's
// own icon is bytes cairn has never read, so there is no digest to offer and
// inventing a name for it would 404.
func TestAnOperatorsOwnIconIsLeftAlone(t *testing.T) {
	for _, p := range []string{
		"/assets/my-icon.png",
		"/media/shot.png",
		"https://cdn.example.org/icon.png",
		"/static/not-a-file-cairn-ships.png",
	} {
		if got := AppURL(p); got != BasePath+p && got != p {
			t.Errorf("AppURL(%q) = %q, want it untouched", p, got)
		}
	}
}

// The digest has to be of the file, not merely stable. A constant per name
// would pass every other test here and would never change on an upgrade,
// which is the one thing it exists to do.
func TestTheDigestIsOfTheFilesOwnBytes(t *testing.T) {
	for _, name := range []string{"style.css", "search.js", "icon-192.png"} {
		b, err := Embedded.ReadFile("assets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		ext := path.Ext(name)
		want := "/static/" + strings.TrimSuffix(name, ext) + "." + hex.EncodeToString(sum[:4]) + ext
		if got := AssetURL(name); got != want {
			t.Errorf("AssetURL(%s) = %q, want %q, the digest of what is in the file", name, got, want)
		}
	}
}
