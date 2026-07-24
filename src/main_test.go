package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestSecurityHeadersAndCSPHashes(t *testing.T) {
	m, err := buildModel(testConfig(t), nil)
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

	// the CSP only works if its hashes match the inline fragments the
	// template actually renders; extract them and check
	html := string(m.Pages["fr"].HTML)
	script := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindStringSubmatch(html)
	style := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindStringSubmatch(html)
	if script == nil || style == nil {
		t.Fatal("inline script or style not found in the rendered page")
	}
	if got := cspHash(script[1]); !strings.Contains(csp, got) {
		t.Errorf("rendered inline script hashes to %s, absent from the CSP: layout.tmpl and prePaintScript diverged", got)
	}
	if got := cspHash(style[1]); !strings.Contains(csp, got) {
		t.Errorf("rendered accent style hashes to %s, absent from the CSP", got)
	}
}

func TestLocaleChoiceAndNegotiation(t *testing.T) {
	m, err := buildModel(testConfig(t), nil) // locales: fr, en
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
	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(cfg, nil)
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
