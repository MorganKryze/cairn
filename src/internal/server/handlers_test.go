package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// storeModel loads a config dir, builds the model and installs it as current,
// the state every handler reads.
func storeModel(t *testing.T, files map[string]string) *config.Config {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	current.Store(m)
	return cfg
}

func TestRobotsAndSitemap(t *testing.T) {
	files := map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlocales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	}
	storeModel(t, files)

	rec := httptest.NewRecorder()
	robots(rec, httptest.NewRequest("GET", "/robots.txt", nil))
	if body := rec.Body.String(); !strings.Contains(body, "Allow: /") || !strings.Contains(body, "Sitemap: https://tools.example.org/sitemap.xml") {
		t.Errorf("robots (indexable) = %q", body)
	}

	rec = httptest.NewRecorder()
	sitemap(rec, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "<loc>https://tools.example.org/en/</loc>") || !strings.Contains(body, "en/pad/") {
		t.Errorf("sitemap = %q", body)
	}

	// index: false flips both to "stay away".
	files["site.yaml"] = "url: https://tools.example.org\nlocales: [en]\nindex: false\n"
	storeModel(t, files)

	rec = httptest.NewRecorder()
	robots(rec, httptest.NewRequest("GET", "/robots.txt", nil))
	if body := rec.Body.String(); !strings.Contains(body, "Disallow: /") {
		t.Errorf("robots (noindex) = %q, want Disallow", body)
	}
	rec = httptest.NewRecorder()
	sitemap(rec, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("sitemap (noindex) = %d, want 404", rec.Code)
	}
}

func TestManifest(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "title: Tools\nlocales: [en]\ntheme: {accent: \"#0055ff\"}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	rec := httptest.NewRecorder()
	manifest(rec, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("content-type = %q", ct)
	}
	var mf struct {
		Name       string `json:"name"`
		ThemeColor string `json:"theme_color"`
		Icons      []struct{ Src, Sizes, Type string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mf); err != nil {
		t.Fatal(err)
	}
	if mf.Name != "Tools" || mf.ThemeColor != "#0055ff" {
		t.Errorf("manifest = %+v", mf)
	}
	if len(mf.Icons) != 1 || mf.Icons[0].Sizes != "180x180" || mf.Icons[0].Type != "image/png" {
		t.Errorf("icons = %+v", mf.Icons)
	}
}

func TestNoListing(t *testing.T) {
	h := noListing(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "file") }))
	// After StripPrefix a directory request is the empty path or ends in /.
	for _, p := range []string{"", "sub/"} {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/media/x", nil)
		r.URL.Path = p
		h.ServeHTTP(rec, r)
		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q = %d, want 404", p, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/media/x.png", nil)
	r.URL.Path = "x.png"
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK || rec.Body.String() != "file" {
		t.Errorf("file passthrough = %d %q", rec.Code, rec.Body.String())
	}
}

func TestCacheControl(t *testing.T) {
	rec := httptest.NewRecorder()
	cacheControl(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, httptest.NewRequest("GET", "/static/x", nil))
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("cache-control = %q", cc)
	}
}

func TestBaseURL(t *testing.T) {
	// No site.url: siteBase falls back to what the request suggests.
	storeModel(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "cairn.local"
	if got := siteBase(r); got != "http://cairn.local" {
		t.Errorf("siteBase (no url, http) = %q", got)
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := baseURL(r); got != "https://cairn.local" {
		t.Errorf("baseURL (forwarded https) = %q", got)
	}
	// A configured public URL wins over the request.
	storeModel(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if got := siteBase(httptest.NewRequest("GET", "/", nil)); got != "https://tools.example.org" {
		t.Errorf("siteBase (configured) = %q", got)
	}
}

func TestConfigHelpers(t *testing.T) {
	cfg := sample(t)
	if n := config.CountServices(cfg); n != 2 {
		t.Errorf("config.CountServices = %d, want 2", n)
	}
	for _, tc := range []struct {
		set  string
		want time.Duration
	}{
		{"", 60 * time.Second}, // unset -> default
		{"30s", 30 * time.Second},
		{"2s", 60 * time.Second}, // below the 5s floor -> default
	} {
		cfg.Site.Status.Interval = tc.set
		if d := cfg.StatusInterval(); d != tc.want {
			t.Errorf("StatusInterval(%q) = %s, want %s", tc.set, d, tc.want)
		}
	}
}
