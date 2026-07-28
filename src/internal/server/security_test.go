package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestSecurityHeadersAndCSPHashes(t *testing.T) {
	m, err := render.BuildModel(sample(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	current.Store(m)
	rec := httptest.NewRecorder()
	secureHeaders(http.HandlerFunc(home)).ServeHTTP(rec, httptest.NewRequest("GET", "/fr/", nil))
	h := rec.Result().Header
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := h.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	csp := h.Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("csp = %q, missing base directives", csp)
	}
}

func TestLocaleChoiceAndNegotiation(t *testing.T) {
	m, err := render.BuildModel(sample(t), nil) // locales: fr, en
	if err != nil {
		t.Fatal(err)
	}
	current.Store(m)

	rec := httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/en/", nil))
	if len(rec.Result().Cookies()) != 0 {
		t.Error("a plain visit should not pin the language")
	}

	rec = httptest.NewRecorder()
	home(rec, httptest.NewRequest("GET", "/en/?choose", nil))
	resp := rec.Result()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/en/" {
		t.Errorf("choose = %d %s, want 302 to /en/", resp.StatusCode, resp.Header.Get("Location"))
	}
	var pinned *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "locale" {
			pinned = c
		}
	}
	if pinned == nil || pinned.Value != "en" {
		t.Fatalf("cookie = %v, want locale=en", pinned)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	root(rec, req)
	if loc := rec.Result().Header.Get("Location"); loc != "/en/" {
		t.Errorf("browser language: redirected to %s, want /en/", loc)
	}

	req.AddCookie(&http.Cookie{Name: "locale", Value: "fr"})
	rec = httptest.NewRecorder()
	root(rec, req)
	if loc := rec.Result().Header.Get("Location"); loc != "/fr/" {
		t.Errorf("explicit choice should beat the browser language, got %s", loc)
	}
}

func TestLocaleCookieSecureFollowsScheme(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := render.BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	current.Store(m)

	for _, tc := range []struct {
		proto  string
		secure bool
	}{{"", false}, {"https", true}} {
		r := httptest.NewRequest("GET", "/en/?choose", nil)
		if tc.proto != "" {
			r.Header.Set("X-Forwarded-Proto", tc.proto)
		}
		w := httptest.NewRecorder()
		home(w, r)
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "locale" {
			t.Fatalf("proto %q: cookies = %v, want one locale cookie", tc.proto, cookies)
		}
		if cookies[0].Secure != tc.secure {
			t.Errorf("proto %q: Secure = %v, want %v", tc.proto, cookies[0].Secure, tc.secure)
		}
	}
}
