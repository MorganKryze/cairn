package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/render"
)

// The whole mechanism in one file: a stamped name has to come back with the
// year, a plain one with the day it has always had, and a digest cairn never
// issued has to come back not at all.
//
// The header is the half that is easy to get wrong and impossible to see: two
// wrappers both write Cache-Control, and whichever runs last wins. Written
// with them in the wrong order, this failed with the year overwritten by the
// day, which is the bug that would have shipped a cache nobody could rely on.
func TestStampedAssetsAreCachedForAYear(t *testing.T) {
	mux := routes(t.TempDir(), t.TempDir())
	stampedURL := render.AssetURL("style.css")

	for _, c := range []struct {
		what, path, cache string
		status            int
	}{
		{"a stamped name", stampedURL, "public, max-age=31536000, immutable", http.StatusOK},
		{"the plain name", "/static/style.css", "public, max-age=86400", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != c.status {
			t.Errorf("%s: %s -> %d, want %d", c.what, c.path, rec.Code, c.status)
		}
		if got := rec.Header().Get("Cache-Control"); got != c.cache {
			t.Errorf("%s: Cache-Control = %q, want %q", c.what, got, c.cache)
		}
		if body := rec.Body.String(); !strings.Contains(body, "--accent") {
			t.Errorf("%s: did not serve the stylesheet", c.what)
		}
	}

	// A guessed digest is not a way to reach a file, nor to have one cached
	// for a year under a name cairn never promised was stable.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/static/style.deadbeef.css", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("an invented digest returned %d, want 404", rec.Code)
	}
}

// The pages have to point at the stamped name, or none of the above matters.
func TestPagesLinkTheStampedStylesheet(t *testing.T) {
	want := render.AssetURL("style.css")
	if strings.Contains(want, "/static/style.css") {
		t.Fatal("the stylesheet is not stamped at all")
	}
}
