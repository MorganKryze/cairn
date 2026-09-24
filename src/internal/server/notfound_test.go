package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/render"
)

func TestAMissingAddressGetsCairnsOwnPage(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en, fr]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	h := Handler(t.TempDir(), t.TempDir())

	for _, c := range []struct {
		path, accept, lang, title string
	}{
		{"/en/nope/", "fr", "en", "Page not found"},                   // the path's language beats the browser's
		{"/fr/nope/", "", "fr", "Page introuvable"},                   // a locale named in the path
		{"/assets/missing.png", "fr", "fr", "Page introuvable"},       // a file tree, negotiated
		{"/static/nothing.css", "", "en", "Page not found"},           // cairn's own tree
		{"/nowhere/at/all", "de, fr;q=0.5", "fr", "Page introuvable"}, // no locale in the path at all
	} {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		if c.accept != "" {
			req.Header.Set("Accept-Language", c.accept)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", c.path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: Content-Type %q, want html", c.path, ct)
		}
		if !strings.Contains(body, `<html lang="`+c.lang+`"`) || !strings.Contains(body, c.title) {
			t.Errorf("%s: want the %s page titled %q, got:\n%.300s", c.path, c.lang, c.title, body)
		}
		// The plain-text body Go wrote first must not trail the page.
		if strings.Contains(body, "404 page not found") {
			t.Errorf("%s: Go's own text is still in the body", c.path)
		}
		if rec.Header().Get("Content-Security-Policy") == "" {
			t.Errorf("%s: the page went out without the hardening headers", c.path)
		}
	}
}

// Outside the mount point the request never reaches the site's routes, and a
// visitor who dropped the prefix is exactly the one who needs a way back.
func TestAPathOutsideTheMountGetsThePageToo(t *testing.T) {
	withBase(t, "/cairn")
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	h := Handler(t.TempDir(), t.TempDir())

	for _, path := range []string{"/elsewhere", "/cairn/en/nope/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Page not found") {
			t.Errorf("%s: %d, want cairn's 404 page", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `href="/cairn/en/"`) {
			t.Errorf("%s: the way back has lost the prefix", path)
		}
	}
}

// A config that never loaded still gets a page, so a visitor who mistypes an
// address on a fresh install is not dropped onto Go's plain text.
func TestTheGettingStartedPageHasOneToo(t *testing.T) {
	current.Store(render.StarterModel())
	rec := httptest.NewRecorder()
	Handler(t.TempDir(), t.TempDir()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/en/nope/", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Page not found") {
		t.Errorf("got %d, want cairn's 404 page", rec.Code)
	}
}

func TestAnythingButA404PassesUntouched(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	h := Handler(t.TempDir(), t.TempDir())
	for path, want := range map[string]int{"/en/": http.StatusOK, "/": http.StatusFound, "/healthz": http.StatusOK} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want || strings.Contains(rec.Body.String(), "Page not found") {
			t.Errorf("%s: %d, want a plain %d", path, rec.Code, want)
		}
	}
}
