package render

import (
	"regexp"
	"strings"
	"testing"
)

var hashed = regexp.MustCompile(`^/static/style\.[0-9a-f]{8}\.css$`)

// The name carries a digest of what is inside the file, which is the whole
// mechanism: the page is served no-cache and always fresh, so the moment an
// asset changes the page points somewhere the browser has never been.
func TestAssetURLCarriesTheContentDigest(t *testing.T) {
	got := AssetURL("style.css")
	if !hashed.MatchString(got) {
		t.Fatalf("AssetURL(style.css) = %q, want /static/style.<8 hex>.css", got)
	}
	if AssetURL("style.css") != got {
		t.Error("two calls disagreed, so the digest is not of the content")
	}
	if AssetURL("search.js") == got {
		t.Error("two different files share a url")
	}
}

// A name cairn does not serve keeps the plain path rather than inventing a
// digest for it, so a typo fails loudly at the 404 instead of silently
// resolving to something.
func TestAssetURLLeavesAnUnknownNameAlone(t *testing.T) {
	if got := AssetURL("nope.css"); got != "/static/nope.css" {
		t.Errorf("AssetURL(nope.css) = %q, want the plain path", got)
	}
}

// The font is the deliberate exception, and this is here so that stamping it
// later is a decision rather than an accident. style.css asks for it with a
// relative url() of its own; a stamped <link rel=preload> beside an unstamped
// url() is two different addresses for one file, which costs a wasted 64 KB
// download rather than saving anything. Stamp it only together with the
// stylesheet's own bytes.
func TestTheFontKeepsItsPlainName(t *testing.T) {
	if got := AssetURL("fonts/fraunces.woff2"); got != "/static/fonts/fraunces.woff2" {
		t.Errorf("the font is stamped (%q) while style.css still asks for the plain name", got)
	}
}

// The digest has to sit before the extension, not after it, or the file is
// served as the wrong type: style.css.a1b2c3d4 is not a stylesheet to any
// content-type table.
func TestTheDigestSitsBeforeTheExtension(t *testing.T) {
	for _, name := range []string{"style.css", "search.js", "favicon.svg", "icon-192.png"} {
		got := AssetURL(name)
		ext := name[strings.LastIndex(name, "."):]
		if !strings.HasSuffix(got, ext) {
			t.Errorf("AssetURL(%s) = %q, which no longer ends in %s", name, got, ext)
		}
	}
}

// AssetPath is the other half: what the server hands back to the file it has
// to open, and it must refuse a digest that is not the one it computed, so a
// guessed url cannot pull a file out from under the cache rules.
func TestAssetPathResolvesOnlyTheDigestItIssued(t *testing.T) {
	url := strings.TrimPrefix(AssetURL("style.css"), "/static/")
	if got, ok := AssetPath(url); !ok || got != "style.css" {
		t.Errorf("AssetPath(%q) = %q, %v; want style.css, true", url, got, ok)
	}
	if _, ok := AssetPath("style.deadbeef.css"); ok {
		t.Error("a digest cairn never issued resolved anyway")
	}
	// The plain name is not this function's business: the file server still
	// answers it directly, on the shorter cache.
	if _, ok := AssetPath("style.css"); ok {
		t.Error("the plain name resolved through the digest table")
	}
}
