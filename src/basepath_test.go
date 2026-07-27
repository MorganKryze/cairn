package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestNormalizeBase(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"", ""},
		{"/", ""},
		{"cairn", "/cairn"},
		{"/cairn", "/cairn"},
		{"/cairn/", "/cairn"},
		{"  /cairn/  ", "/cairn"},
		{"tools/cairn", "/tools/cairn"},
	} {
		if got := normalizeBase(c.in); got != c.want {
			t.Errorf("normalizeBase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// withBase installs a base path for one test and restores it after, so the
// package-level default ("" = domain root) never leaks between tests.
func withBase(t *testing.T, p string) {
	t.Helper()
	prev := basePath
	basePath = p
	t.Cleanup(func() { basePath = prev })
}

// rootAbsolute finds every generated URL that starts at the domain root; under
// a base path each one must carry the prefix or the browser leaves the app.
var rootAbsolute = regexp.MustCompile(`(?:href|src)="(/[^"]*)"`)

func TestBasePathPrefixesEveryGeneratedURL(t *testing.T) {
	withBase(t, "/cairn")
	files := map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [en]\n" +
			"logo: /assets/logo.png\nfavicon: /assets/fav.png\n" +
			"about: Hello.\nlinks: [{label: Wiki, url: https://wiki.example.org, icon: /assets/w.png}]\n" +
			"pages: [{id: legal, title: Legal, body: Text.}]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, details: More.}\n",
	}
	storeModel(t, files)

	for _, path := range []string{"en", "en/pad", "en/legal"} {
		page, ok := current.Load().Pages[path]
		if !ok {
			t.Fatalf("page %q missing", path)
		}
		for _, m := range rootAbsolute.FindAllStringSubmatch(string(page.HTML), -1) {
			if !strings.HasPrefix(m[1], "/cairn/") {
				t.Errorf("page %q: %q escapes the base path", path, m[1])
			}
		}
	}
}

func TestBasePathRedirectsStayInside(t *testing.T) {
	withBase(t, "/cairn")
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})

	// the site root redirects to a locale that is still under the prefix
	rec := httptest.NewRecorder()
	root(rec, httptest.NewRequest("GET", "/", nil))
	if got := rec.Header().Get("Location"); got != "/cairn/en/" {
		t.Errorf("root redirect = %q, want /cairn/en/", got)
	}

	// a missing trailing slash is normalized without dropping the prefix
	rec = httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/en", nil))
	if got := rec.Header().Get("Location"); got != "/cairn/en/" {
		t.Errorf("slash redirect = %q, want /cairn/en/", got)
	}

	// the language cookie is scoped to the mount point, not the whole domain
	rec = httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/en/?choose", nil))
	cookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "Path=/cairn/") {
		t.Errorf("cookie = %q, want Path=/cairn/", cookie)
	}
}

func TestBasePathCanonicalAndSitemap(t *testing.T) {
	withBase(t, "/cairn")
	storeModel(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlocales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})

	html := string(current.Load().Pages["en"].HTML)
	if !strings.Contains(html, `<link rel="canonical" href="https://tools.example.org/cairn/en/">`) {
		t.Error("canonical URL misses the base path")
	}

	rec := httptest.NewRecorder()
	sitemap(rec, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if body := rec.Body.String(); !strings.Contains(body, "<loc>https://tools.example.org/cairn/en/</loc>") {
		t.Errorf("sitemap = %q", body)
	}

	rec = httptest.NewRecorder()
	robots(rec, httptest.NewRequest("GET", "/robots.txt", nil))
	if body := rec.Body.String(); !strings.Contains(body, "Sitemap: https://tools.example.org/cairn/sitemap.xml") {
		t.Errorf("robots = %q", body)
	}
}

// The router strips the prefix back off, so the proxy in front needs no
// rewriting; /healthz stays at the root for container healthchecks.
func TestMountRoutes(t *testing.T) {
	withBase(t, "/cairn")
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	inner := http.NewServeMux()
	inner.HandleFunc("GET /healthz", healthz)
	inner.HandleFunc("GET /{$}", root)
	inner.HandleFunc("GET /", home)
	h := mount(inner)

	for _, c := range []struct {
		path string
		want int
	}{
		{"/cairn/en/", http.StatusOK},
		{"/cairn/", http.StatusFound},            // -> /cairn/en/
		{"/cairn", http.StatusTemporaryRedirect}, // ServeMux adds the slash
		{"/healthz", http.StatusOK},              // infra probe, outside the prefix
		{"/cairn/healthz", http.StatusOK},        // and through the proxy too
		{"/en/", http.StatusNotFound},            // outside the mount point
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != c.want {
			t.Errorf("GET %s = %d, want %d", c.path, rec.Code, c.want)
		}
	}
}

// Without the flag nothing changes: the default deployment stays at the root.
func TestNoBasePathIsUnchanged(t *testing.T) {
	withBase(t, "")
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html := string(current.Load().Pages["en"].HTML)
	if !strings.Contains(html, `href="/static/style.css"`) {
		t.Error("root deployment should keep bare /static/ URLs")
	}
	rec := httptest.NewRecorder()
	root(rec, httptest.NewRequest("GET", "/", nil))
	if got := rec.Header().Get("Location"); got != "/en/" {
		t.Errorf("root redirect = %q, want /en/", got)
	}
}
