package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// The route table says which URLs exist. Nothing else fails if one disappears
// in a bad merge, so this walks every path cairn is supposed to answer.
func TestRouteTable(t *testing.T) {
	cfgDir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlocales: [en]\npages: [{id: legal, title: Legal, body: Text.}]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n",
		"custom.css":    ":root{--x:1}\n",
	})
	assetsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assetsDir, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "icons", "pad.svg"), []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "media", "shot.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	Store(mustModel(t, cfgDir))
	h := routes(cfgDir, assetsDir)

	for _, c := range []struct {
		path string
		want int
	}{
		{"/", http.StatusFound},                  // to the negotiated locale
		{"/en/", http.StatusOK},                  // a home
		{"/en/pad/", http.StatusOK},              // a service detail
		{"/en/legal/", http.StatusOK},            // a hosted page
		{"/en/nope/", http.StatusNotFound},       // an unknown page under a locale
		{"/healthz", http.StatusOK},              // liveness
		{"/readyz", http.StatusOK},               // readiness
		{"/robots.txt", http.StatusOK},           //
		{"/sitemap.xml", http.StatusOK},          //
		{"/manifest.webmanifest", http.StatusOK}, //
		{"/favicon.ico", http.StatusOK},          // for the tools that skip the html
		{"/custom.css", http.StatusOK},           // the operator's stylesheet
		{"/static/style.css", http.StatusOK},     // embedded
		{"/assets/icons/pad.svg", http.StatusOK}, // mounted
		{"/media/shot.txt", http.StatusOK},       // next to the yaml
		{"/assets/", http.StatusNotFound},        // never a directory index
		{"/media/", http.StatusNotFound},         //
		{"/static/does-not-exist", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != c.want {
			t.Errorf("GET %s = %d, want %d", c.path, rec.Code, c.want)
		}
	}
}

// Static assets are cached for a day, pages never are: a config edit has to
// reach a returning visitor within the poll interval, not a day later.
func TestCachingPolicyPerRoute(t *testing.T) {
	cfgDir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	Store(mustModel(t, cfgDir))
	h := routes(cfgDir, t.TempDir())

	for _, c := range []struct{ path, want string }{
		{"/static/style.css", "public, max-age=86400"},
		{"/en/", "no-cache"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if got := rec.Header().Get("Cache-Control"); got != c.want {
			t.Errorf("%s Cache-Control = %q, want %q", c.path, got, c.want)
		}
	}
}

// A locale with no trailing slash is redirected rather than served, so one
// page never answers on two URLs.
func TestHomeRedirects(t *testing.T) {
	cfgDir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	Store(mustModel(t, cfgDir))

	rec := httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/en", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/en/" {
		t.Errorf("missing slash = %d %q, want 301 /en/", rec.Code, rec.Header().Get("Location"))
	}

	// an unknown single segment is a stale bookmark, not a page: send it home
	rec = httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/de/", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Errorf("unknown locale = %d %q, want 302 /", rec.Code, rec.Header().Get("Location"))
	}

	// an unchanged page answers 304 rather than resending its bytes
	rec = httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/en/", nil))
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on a page response")
	}
	r := httptest.NewRequest("GET", "/en/", nil)
	r.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	home(rec, r)
	if rec.Code != http.StatusNotModified {
		t.Errorf("matching ETag = %d, want 304", rec.Code)
	}
	if n := rec.Body.Len(); n != 0 {
		t.Errorf("304 carried a body of %d bytes", n)
	}
}

// mustModel builds the model a directory would be served as.
func mustModel(t *testing.T, dir string) *render.Model {
	t.Helper()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A listen address the OS refuses has to come back as an error rather than a
// panic, so the process can log something an operator can act on.
func TestServeRejectsABadAddress(t *testing.T) {
	if err := Serve("not-an-address", t.TempDir(), t.TempDir()); err == nil {
		t.Error("a malformed listen address was accepted")
	}
}

// Behind a proxy the scheme comes from a header, not the connection, or every
// canonical URL on an https site would claim to be http.
func TestBaseURLFollowsTheProxy(t *testing.T) {
	plain := httptest.NewRequest("GET", "/", nil)
	plain.Host = "tools.example.org"
	if got := baseURL(plain); got != "http://tools.example.org" {
		t.Errorf("plain request = %q", got)
	}

	fwd := httptest.NewRequest("GET", "/", nil)
	fwd.Host = "tools.example.org"
	fwd.Header.Set("X-Forwarded-Proto", "https")
	if got := baseURL(fwd); got != "https://tools.example.org" {
		t.Errorf("forwarded request = %q", got)
	}

	direct := httptest.NewRequest("GET", "/", nil)
	direct.Host = "tools.example.org"
	direct.TLS = &tls.ConnectionState{}
	if got := baseURL(direct); got != "https://tools.example.org" {
		t.Errorf("direct TLS request = %q", got)
	}
}
