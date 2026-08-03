package render

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"strings"
)

// Assets are served under a name that carries a digest of what is inside them,
// so `style.css` goes out as `style.a1b2c3d4.css`.
//
// The problem it solves is the one an operator meets on every upgrade: pages
// are served no-cache and update the moment cairn restarts, while the
// stylesheet beside them was cached for a day under a name that never changes.
// The page was new, the CSS was yesterday's, and only a hard refresh fixed it.
//
// Since the name changes exactly when the bytes change, the page that points
// at it is already fresh, and a browser meeting a name it has never seen has
// no choice but to fetch it. That is also what makes it safe to cache these
// for a year: nothing has to expire, because nothing is ever replaced under
// the same name.
//
// The digest is in the name rather than in a query string on purpose. A query
// is easier to write, and it leaves an intermediary free to key its cache on
// the path alone, which would pin a stale file for the whole year rather than
// for a day. A different name is a different object to everything in the path.
//
// One gap, named rather than hidden: the display font keeps its plain name.
// The stylesheet reaches it with a relative url(), and no stamping here can
// follow into a CSS file's own bytes, so stamping the <link rel=preload> alone
// would leave the two pointing at different urls and the browser fetching 64
// KB twice, having preloaded a file the stylesheet then does not ask for.
// Covering it properly means rewriting the stylesheet on the way out; it has
// changed once in the project's life, so it waits until that is worth it.
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
		// Eight hex is 32 bits. This is a cache key, not a security boundary:
		// the file it names is decided by the table below, so a collision
		// would serve one of two files cairn itself ships, and never anything
		// an outsider chose.
		digest := hex.EncodeToString(sum[:4])
		ext := path.Ext(name)
		stamped := strings.TrimSuffix(name, ext) + "." + digest + ext
		assetURL[name] = stamped
		assetReal[stamped] = name
		return nil
	})
}

// AssetURL is the path a page should point at for one of cairn's own assets.
// A name cairn does not ship comes back untouched, so a typo arrives at the
// 404 it deserves instead of resolving to something else.
func AssetURL(name string) string {
	if stamped, ok := assetURL[name]; ok {
		return BasePath + "/static/" + stamped
	}
	return BasePath + "/static/" + name
}

// AssetPath turns a stamped name back into the file to open, and refuses a
// digest cairn did not issue. The plain name is deliberately not resolved
// here: the file server still answers it directly, on the shorter cache, so
// anything that hardcoded /static/style.css goes on working.
func AssetPath(stamped string) (string, bool) {
	real, ok := assetReal[stamped]
	return real, ok
}
