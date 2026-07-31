package server

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/fs"
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

// X-Forwarded-Proto is attacker-controlled until a proxy overwrites it, and
// what it builds is published in the sitemap. Anything but the two schemes
// cairn can be reached over is dropped for what this connection actually is,
// and every value that reaches the document is escaped: an unescaped "&" makes
// the whole sitemap unparseable, so one crafted request would take the site out
// of every search index rather than merely poison one URL.
func TestSitemapStaysWellFormedXML(t *testing.T) {
	for _, c := range []struct {
		name, siteYAML, forwarded, host, wantLoc string
	}{
		{
			name:      "a scheme that is not a scheme falls back to the connection",
			siteYAML:  "locales: [en]\n",
			forwarded: "x&y",
			host:      "cairn.local",
			wantLoc:   "http://cairn.local/en/",
		},
		{
			name:      "javascript: never becomes the scheme of a published link",
			siteYAML:  "locales: [en]\n",
			forwarded: "javascript",
			host:      "cairn.local",
			wantLoc:   "http://cairn.local/en/",
		},
		{
			// A configured url is the operator's own text and passes validation
			// with an ampersand in it, so escaping is what keeps the document
			// parseable rather than the scheme check.
			name:     "an ampersand in the configured url is escaped, not emitted raw",
			siteYAML: "url: https://a&b.example.org\nlocales: [en]\n",
			host:     "cairn.local",
			wantLoc:  "https://a&b.example.org/en/",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			storeModel(t, map[string]string{
				"site.yaml":     c.siteYAML,
				"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
			})
			r := httptest.NewRequest("GET", "/sitemap.xml", nil)
			r.Host = c.host
			if c.forwarded != "" {
				r.Header.Set("X-Forwarded-Proto", c.forwarded)
			}
			rec := httptest.NewRecorder()
			sitemap(rec, r)

			// Parsed, not grepped: a sitemap that does not parse is a sitemap no
			// crawler reads, however right the text inside it looks.
			var doc struct {
				XMLName xml.Name `xml:"urlset"`
				URLs    []struct {
					Loc string `xml:"loc"`
				} `xml:"url"`
			}
			if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
				t.Fatalf("sitemap does not parse: %v\n%s", err, rec.Body.String())
			}
			if len(doc.URLs) == 0 {
				t.Fatalf("sitemap carries no urls:\n%s", rec.Body.String())
			}
			// xml.Unmarshal has already turned &amp; back into &, so this compares
			// the URL a crawler ends up with, not the bytes on the wire.
			if got := doc.URLs[0].Loc; got != c.wantLoc {
				t.Errorf("first loc = %q, want %q", got, c.wantLoc)
			}
		})
	}
}

// The other direction of the same rule: a proxy terminating TLS is the ordinary
// deployment, and its header still decides the scheme cairn publishes. Refusing
// the header outright would be a fix that broke every site behind a reverse
// proxy, which is most of them.
func TestRobotsHonoursALegitimateForwardedProto(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	for _, c := range []struct{ name, forwarded, want string }{
		{"a proxy terminating TLS", "https", "Sitemap: https://cairn.local/sitemap.xml"},
		{"a proxy that does not", "http", "Sitemap: http://cairn.local/sitemap.xml"},
		{"no proxy at all", "", "Sitemap: http://cairn.local/sitemap.xml"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/robots.txt", nil)
			r.Host = "cairn.local"
			if c.forwarded != "" {
				r.Header.Set("X-Forwarded-Proto", c.forwarded)
			}
			rec := httptest.NewRecorder()
			robots(rec, r)
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("robots.txt = %q, want %q", body, c.want)
			}
		})
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
		Icons      []struct{ Src, Sizes, Type, Purpose string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mf); err != nil {
		t.Fatal(err)
	}
	if mf.Name != "Tools" || mf.ThemeColor != "#0055ff" {
		t.Errorf("manifest = %+v", mf)
	}

	// Chromium will not offer to install a site whose manifest lacks either a
	// 192 or a 512, and Android crops anything not declared maskable. Both are
	// silent failures in a browser, so they are pinned here instead.
	for _, want := range []struct{ sizes, purpose string }{
		{"180x180", ""},
		{"192x192", "any maskable"}, {"512x512", "any maskable"},
	} {
		found := false
		for _, ic := range mf.Icons {
			if ic.Sizes == want.sizes && ic.Purpose == want.purpose && ic.Type == "image/png" {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s icon with purpose %q in %+v", want.sizes, want.purpose, mf.Icons)
		}
	}
}

// The well-known path is the one a link previewer fetches without reading the
// html, so it must never hand cairn's mark to a site that has its own.
func TestFaviconICOYieldsToTheOperator(t *testing.T) {
	static, err := fs.Sub(render.Embedded, "assets")
	if err != nil {
		t.Fatal(err)
	}
	h := faviconICO(static)

	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/favicon.ico", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Errorf("default = %d, %d bytes; want cairn's own ico", rec.Code, rec.Body.Len())
	}

	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\nfavicon: /assets/brand.svg\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/favicon.ico", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/assets/brand.svg" {
		t.Errorf("with a custom favicon = %d %q, want a redirect to it",
			rec.Code, rec.Header().Get("Location"))
	}
}

// End to end through the handler: what the operator wrote is what a phone
// reads. The per-case rules live in render's own tests; what matters here is
// that site.yaml reaches the served json intact.
func TestManifestServesTheOperatorsIcons(t *testing.T) {
	icons := func(t *testing.T, yaml string) []struct{ Src, Sizes, Type, Purpose string } {
		t.Helper()
		storeModel(t, map[string]string{
			"site.yaml":     "title: Tools\nlocales: [en]\n" + yaml,
			"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
		})
		rec := httptest.NewRecorder()
		manifest(rec, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
		var mf struct {
			Icons []struct{ Src, Sizes, Type, Purpose string }
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &mf); err != nil {
			t.Fatal(err)
		}
		return mf.Icons
	}

	// A png we cannot open is offered without a size rather than with a wrong
	// one; cairn's mark never joins it, or our logo lands on their home screen.
	got := icons(t, "favicon: /assets/brand.png\n")
	if len(got) != 1 || got[0].Src != "/assets/brand.png" || got[0].Sizes != "" || got[0].Type != "image/png" {
		t.Errorf("unmeasurable favicon = %+v, want the file alone with no size", got)
	}

	// An explicit list comes back in order, verbatim.
	got = icons(t, "icons:\n  - {src: /assets/a-192.png, sizes: 192x192}\n"+
		"  - {src: /assets/a-512.png, sizes: 512x512, purpose: any maskable}\n")
	if len(got) != 2 {
		t.Fatalf("explicit list = %+v, want 2 entries", got)
	}
	if got[1].Src != "/assets/a-512.png" || got[1].Sizes != "512x512" || got[1].Purpose != "any maskable" {
		t.Errorf("second entry = %+v", got[1])
	}

	// And a typo in it never reaches a phone: it stops the config from loading.
	if _, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nicons:\n  - {src: /assets/a.png, sizes: 512}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})); err == nil {
		t.Error("a malformed icon size was accepted")
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

// Nothing is served until an operator gives a contact: an empty security.txt
// would be a promise cairn cannot keep on their behalf.
func TestSecurityTxtNeedsAContact(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	Store(mustModel(t, dir))
	rec := httptest.NewRecorder()
	securityTxt(rec, httptest.NewRequest("GET", "/.well-known/security.txt", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unconfigured security.txt = %d, want 404", rec.Code)
	}
}

// The three fields cairn fills by itself are the point of serving the file
// rather than letting the operator drop a static one that rots.
func TestSecurityTxtFillsWhatItKnows(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlocales: [fr, en]\n" +
			"security: {contact: mailto:security@example.org, policy: https://example.org/policy}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	Store(mustModel(t, dir))
	rec := httptest.NewRecorder()
	securityTxt(rec, httptest.NewRequest("GET", "/.well-known/security.txt", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Contact: mailto:security@example.org",
		"Policy: https://example.org/policy",
		"Preferred-Languages: fr, en",
		"Canonical: https://tools.example.org/.well-known/security.txt",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("security.txt missing %q, got:\n%s", want, body)
		}
	}
	// Encryption was not configured, so it must not appear at all rather than
	// appear empty: a field with no value is a parse error for RFC 9116.
	if strings.Contains(body, "Encryption:") {
		t.Error("an unset Encryption was emitted anyway")
	}
}

// Expires is the field a hand-written security.txt gets wrong, because it is
// written once and then forgotten. Computed per request, it cannot be stale.
func TestSecurityTxtExpiresInTheFuture(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nsecurity: {contact: mailto:security@example.org}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	Store(mustModel(t, dir))
	rec := httptest.NewRecorder()
	securityTxt(rec, httptest.NewRequest("GET", "/.well-known/security.txt", nil))

	var raw string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if v, ok := strings.CutPrefix(line, "Expires: "); ok {
			raw = v
		}
	}
	if raw == "" {
		t.Fatalf("no Expires line in:\n%s", rec.Body.String())
	}
	when, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("Expires %q is not RFC 3339: %v", raw, err)
	}
	if !when.After(time.Now()) {
		t.Errorf("Expires %s is already past", raw)
	}
}
