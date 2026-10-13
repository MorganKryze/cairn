package render

import (
	"io/fs"
	"reflect"
	"sort"
	"testing"
)

// What the page can load is a budget: every script is a file the CSP lets in,
// every font and image is bytes on a phone, and the image size guard in CI only
// notices them in aggregate. The set is pinned so adding one is a decision
// made in this file, not a side effect of a stylesheet change.
func TestThePageShipsNoNewScriptFontOrImage(t *testing.T) {
	var got []string
	err := fs.WalkDir(Embedded, "assets", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			got = append(got, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	want := []string{
		"assets/about.js", "assets/favicon.ico", "assets/favicon.svg",
		"assets/fonts/fraunces.woff2", "assets/icon-192.png", "assets/icon-512.png",
		"assets/lang.js", "assets/leave.js", "assets/nav.js", "assets/search.js",
		"assets/status.js", "assets/style.css", "assets/theme.js", "assets/toc.js",
		"assets/touch-icon.png",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("embedded assets changed:\n got %q\nwant %q", got, want)
	}
}
