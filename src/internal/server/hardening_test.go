package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	if _, err := status.Fetch(srv.URL); err == nil {
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

	st, err := status.Fetch(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !st["pad"] {
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
