package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
