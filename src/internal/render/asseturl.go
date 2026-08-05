package render

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"strings"
)

// Assets carry a digest of their own bytes in their name: style.a1b2c3d4.css.
// Pages are no-cache and update on restart, so before this an upgrade left the
// new page pointing at yesterday's cached stylesheet until a hard refresh.
//
// A name that changes with the bytes is what makes a year-long cache safe:
// nothing is ever replaced under the same name. In the name rather than in a
// query string, because a query leaves an intermediary free to key its cache
// on the path alone and pin the stale file for that whole year.
//
// The display font keeps its plain name. The stylesheet reaches it with a
// relative url(), which no stamping out here can follow into, so stamping the
// preload alone would make the two disagree and fetch 64 KB twice. Covering it
// means rewriting the stylesheet on the way out, and the font has changed once
// in the project's life.
var (
	assetURL  = map[string]string{} // style.css -> style.a1b2c3d4.css
	assetReal = map[string]string{} // style.a1b2c3d4.css -> style.css
)

func init() {
	_ = fs.WalkDir(Embedded, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := Embedded.ReadFile(p)
		if err != nil {
			return nil
		}
		name := strings.TrimPrefix(p, "assets/")
		// See the note above: a file the stylesheet reaches by itself cannot
		// be stamped from out here without the two disagreeing.
		if strings.HasPrefix(name, "fonts/") {
			return nil
		}
		sum := sha256.Sum256(b)
		// 32 bits, a cache key and not a security boundary: a collision picks
		// between two files cairn itself ships, never one an outsider chose.
		digest := hex.EncodeToString(sum[:4])
		ext := path.Ext(name)
		stamped := strings.TrimSuffix(name, ext) + "." + digest + ext
		assetURL[name] = stamped
		assetReal[stamped] = name
		return nil
	})
}

// AssetURL returns the stamped path for one of cairn's own assets. A name
// cairn does not ship comes back untouched, so a typo reaches its 404.
func AssetURL(name string) string {
	if stamped, ok := assetURL[name]; ok {
		return BasePath + "/static/" + stamped
	}
	return BasePath + "/static/" + name
}

// AssetPath turns a stamped name back into the file to open and refuses a
// digest cairn did not issue. Plain names are not resolved here: the file
// server answers those directly on the shorter cache, so anything that
// hardcoded /static/style.css goes on working.
func AssetPath(stamped string) (string, bool) {
	real, ok := assetReal[stamped]
	return real, ok
}

// stampStatic rewrites a root-absolute path naming one of cairn's own assets,
// and leaves everything else alone. The touch icon and the manifest icons are
// built in Go rather than in a template, so they never meet the asset function
// and would otherwise go out unstamped. An operator's own icon is left as is:
// cairn does not ship those bytes and has no digest to offer.
func stampStatic(p string) string {
	name, ok := strings.CutPrefix(p, "/static/")
	if !ok {
		return p
	}
	stamped, ok := assetURL[name]
	if !ok {
		return p
	}
	return "/static/" + stamped
}
