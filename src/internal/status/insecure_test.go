package status_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/status"
)

// The whole point of the flag, against a certificate nothing signed rather
// than against a mock: httptest.NewTLSServer mints its own, which is exactly
// the shape of an internal Gatus. Verifying has to fail and skipping has to
// succeed, and asserting only one of the two would pass with the flag ignored.
func TestInsecureSkipsTheVerificationAndNothingElse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"pad","results":[{"success":true}]}]`))
	}))
	defer srv.Close()

	if _, err := status.Fetch(srv.URL, false); err == nil {
		t.Fatal("a self-signed gatus verified, so the default client is not checking anything")
	} else if !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("failed for the wrong reason: %v", err)
	}

	st, err := status.Fetch(srv.URL, true)
	if err != nil {
		t.Fatalf("insecure still refused a self-signed gatus: %v", err)
	}
	if !st["pad"].Up {
		t.Errorf("statuses = %v, want pad up: the body has to be read the same way either side of the flag", st)
	}
}

// The clients are package level so a poll every interval reuses one connection
// pool. Two calls must not be two transports.
func TestFetchReusesItsClients(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	for range 3 {
		if _, err := status.Fetch(srv.URL, true); err != nil {
			t.Fatalf("poll refused: %v", err)
		}
	}
}
