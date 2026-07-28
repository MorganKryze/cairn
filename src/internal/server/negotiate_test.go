package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Language negotiation is an HTTP concern: the cookie an explicit choice set,
// then Accept-Language, then the first configured locale.
func TestNegotiate(t *testing.T) {
	locales := []string{"fr", "en"}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Language", "de;q=0.9, en-GB;q=0.8")
	if got := negotiate(r, locales); got != "en" {
		t.Errorf("header negotiation = %q, want en", got)
	}
	r.AddCookie(&http.Cookie{Name: "locale", Value: "fr"})
	if got := negotiate(r, locales); got != "fr" {
		t.Errorf("cookie should win, got %q", got)
	}
	bare := httptest.NewRequest("GET", "/", nil)
	if got := negotiate(bare, locales); got != "fr" {
		t.Errorf("default = %q, want fr", got)
	}
}
