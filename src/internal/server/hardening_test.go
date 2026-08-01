package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// HSTS is sent only once the visit is already https: on the plain-http LAN
// deployments cairn also serves, it would strand the site.
func TestHSTSFollowsTheScheme(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	h := secureHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("plain http carried HSTS = %q", got)
	}

	rec = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	h.ServeHTTP(rec, r)
	if got := rec.Header().Get("Strict-Transport-Security"); !strings.HasPrefix(got, "max-age=") {
		t.Errorf("https missed HSTS, got %q", got)
	}

	// the rest of the hardening headers ride on every response
	for _, k := range []string{"X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options", "Permissions-Policy", "Content-Security-Policy", "Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy"} {
		if rec.Header().Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
}

// mount answers /healthz, /readyz and the 404 for everything outside the prefix
// at the domain root itself, whatever -base-path says. With secureHeaders
// wrapped inside mount those three came back bare: no COOP, no CORP and, on the
// one visit where it counts, no HSTS. The bare host is exactly where a first
// visit lands, and a header that never arrives there cannot pin the scheme
// before anyone gets to downgrade it. Both deployments are walked because the
// point is that they stop differing.
func TestEveryAnswerCarriesTheHardeningHeaders(t *testing.T) {
	for _, base := range []string{"", "/cairn"} {
		name := "at the domain root"
		if base != "" {
			name = "under " + base
		}
		t.Run(name, func(t *testing.T) {
			withBase(t, base)
			cfgDir := testutil.WriteFiles(t, map[string]string{
				"site.yaml":     "locales: [en]\n",
				"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
			})
			Store(mustModel(t, cfgDir))
			h := handler(cfgDir, t.TempDir())

			// The three the outer mux owns, plus a page to show the site itself
			// never lost them.
			for _, path := range []string{"/healthz", "/readyz", "/nothing-is-here", base + "/en/"} {
				r := httptest.NewRequest("GET", path, nil)
				r.Header.Set("X-Forwarded-Proto", "https")
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, r)
				for _, k := range []string{
					"Strict-Transport-Security", "Cross-Origin-Opener-Policy",
					"Cross-Origin-Resource-Policy", "Content-Security-Policy",
					"X-Content-Type-Options", "X-Frame-Options",
					"Referrer-Policy", "Permissions-Policy",
				} {
					if rec.Header().Get(k) == "" {
						t.Errorf("GET %s came back with no %s", path, k)
					}
				}
			}
		})
	}
}

// The same probe over a real connection, in both deployments, compared header
// for header. Under -base-path the root /healthz answered from outside both
// wrappers while the default deployment's answered from inside them, so the two
// sent different headers for the same URL and nobody chose that. Recorders
// cannot say this: it is the response as a client receives it that has to match.
func TestProbesAnswerAlikeInBothDeployments(t *testing.T) {
	seen := map[string]http.Header{}
	for _, base := range []string{"", "/cairn"} {
		func() {
			withBase(t, base)
			cfgDir := testutil.WriteFiles(t, map[string]string{
				"site.yaml":     "locales: [en]\n",
				"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
			})
			Store(mustModel(t, cfgDir))
			srv := httptest.NewServer(handler(cfgDir, t.TempDir()))
			defer srv.Close()

			// DisableCompression, or Go asks and unwraps behind our back and the
			// wire becomes the one thing this test cannot see.
			c := &http.Client{Transport: &http.Transport{DisableCompression: true}}
			req, err := http.NewRequest("GET", srv.URL+"/healthz", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept-Encoding", "gzip")
			resp, err := c.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("base %q: /healthz = %d, want 200", base, resp.StatusCode)
			}
			// A probe asked for gzip and got three plain bytes, in both
			// deployments and by construction rather than by luck: healthz sets
			// no Content-Type, compress reads the one on the header before
			// net/http sniffs one, and an empty type is not compressible. Moving
			// compress outside mount therefore costs the probes nothing.
			if got := resp.Header.Get("Content-Encoding"); got != "" {
				t.Errorf("base %q: a three-byte probe came back Content-Encoding %q", base, got)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "ok\n" {
				t.Errorf("base %q: /healthz said %q", base, body)
			}
			resp.Header.Del("Date")
			seen[base] = resp.Header
		}()
	}
	for k, want := range seen[""] {
		if got := seen["/cairn"][k]; !slices.Equal(got, want) {
			t.Errorf("/healthz sent %s: %q at the domain root, %q under a base path", k, want, got)
		}
	}
	for k := range seen["/cairn"] {
		if _, ok := seen[""][k]; !ok {
			t.Errorf("/healthz sent %s only under a base path", k)
		}
	}
}

// A Gatus that never stops talking must not be able to eat the process: the
// body is bounded, so an oversized answer fails instead of growing forever.
func TestGatusBodyIsBounded(t *testing.T) {
	var endpoints []map[string]any
	for i := range 40000 {
		endpoints = append(endpoints, map[string]any{
			"name":    fmt.Sprintf("service-%d-%s", i, strings.Repeat("x", 300)),
			"results": []map[string]any{{"success": true}},
		})
	}
	blob, err := json.Marshal(endpoints)
	if err != nil {
		t.Fatal(err)
	}
	if len(blob) <= 8<<20 {
		t.Fatalf("test payload is only %d bytes, raise it above the 8 MiB cap", len(blob))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(blob)
	}))
	defer srv.Close()

	if _, err := status.Fetch(status.Source{URL: srv.URL}); err == nil {
		t.Error("an over-sized gatus answer was accepted, the cap is not holding")
	}
}

// The ordinary case still decodes, so the cap does not truncate real answers.
func TestGatusReadsANormalAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/endpoints/statuses" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`[{"name":"pad","results":[{"success":false},{"success":true}]}]`))
	}))
	defer srv.Close()

	st, err := status.Fetch(status.Source{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if st["pad"].Level != status.LevelUp {
		t.Errorf("statuses = %v, want pad up (the last result wins)", st)
	}
}

// html/template strips the "data-" prefix before guessing an attribute's type,
// so a name like data-one reads as an on* event handler and its value comes
// back JSON-quoted. The search announcement carries plural forms in data
// attributes; this pins the naming that keeps them literal.
func TestLiveRegionAttributesAreNotJSEscaped(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html := string(current.Load().Pages["en"].HTML)
	for _, want := range []string{`data-single="1 result"`, `data-plural="%d results"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s in the live region", want)
		}
	}
	if strings.Contains(html, "&#34;1 result&#34;") {
		t.Error("the count string was JS-escaped: an attribute is being read as an on* handler")
	}
}

// A container whose config never loaded serves the getting-started page. It is
// alive, so /healthz says so and a liveness probe leaves it running; it is not
// serving the operator's site, so /readyz says that plainly instead of letting
// a monitor sit green on "Almost there".
func TestReadinessSplitsFromLiveness(t *testing.T) {
	current.Store(render.StarterModel())
	rec := httptest.NewRecorder()
	healthz(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz on a stand-in page = %d, want 200: a liveness probe must not restart it", rec.Code)
	}
	rec = httptest.NewRecorder()
	readyz(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz with no valid config = %d, want 503", rec.Code)
	}

	// once a real config lands, both agree
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	for name, h := range map[string]http.HandlerFunc{"healthz": healthz, "readyz": readyz} {
		rec = httptest.NewRecorder()
		h(rec, httptest.NewRequest("GET", "/"+name, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s with a valid config = %d, want 200", name, rec.Code)
		}
	}
}

// The two cross-origin headers carry values chosen rather than copied, so the
// values themselves are worth pinning. COEP is deliberately absent: it would
// demand a CORP header from every remote image, which is exactly what the
// CDN icon slugs and the operators' own image hosts do not send.
func TestCrossOriginHeadersKeepTheirChosenValues(t *testing.T) {
	Store(mustModel(t, testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})))
	h := secureHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if got := rec.Header().Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Errorf("COOP = %q, want same-origin", got)
	}
	// same-site keeps a sibling subdomain able to show a cairn icon, which is
	// how self-hosters lay their services out.
	if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "same-site" {
		t.Errorf("CORP = %q, want same-site", got)
	}
	if got := rec.Header().Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Errorf("COEP is set to %q; it breaks every remote icon", got)
	}
}
