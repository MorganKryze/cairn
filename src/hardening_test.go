package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	for _, k := range []string{"X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options", "Permissions-Policy", "Content-Security-Policy"} {
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

	if _, err := fetchStatuses(srv.URL); err == nil {
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

	st, err := fetchStatuses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !st["pad"] {
		t.Errorf("statuses = %v, want pad up (the last result wins)", st)
	}
}
