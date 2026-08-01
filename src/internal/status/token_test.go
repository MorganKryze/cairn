package status_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/status"
)

// The token is named by the path of a file the platform mounts (a Kubernetes
// Secret, a docker secret, a Vault agent), never written in site.yaml. It is
// read on every poll rather than cached, so a rotated secret takes effect
// without a restart.
func TestTheTokenTravelsInTheAuthorizationHeader(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	// Trailing newline on purpose: every editor and every kubectl create
	// secret --from-file leaves one, and a header carrying it is a header the
	// far end rejects.
	if err := os.WriteFile(file, []byte("s3cr3t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ scheme, want string }{
		{"", "Bearer s3cr3t"},
		{"OAuth", "OAuth s3cr3t"},
	} {
		var seen string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get("Authorization")
			io.WriteString(w, `[{"n":"a","s":"up"}]`)
		}))
		_, err := status.Fetch(status.Source{Provider: "json", URL: srv.URL,
			TokenFile: file, TokenScheme: c.scheme,
			Map: status.Mapping{Key: "n", State: "s", Up: []string{"up"}}})
		srv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if seen != c.want {
			t.Errorf("scheme %q sent %q, want %q", c.scheme, seen, c.want)
		}
	}
}

// A poll error is logged with the address, so anything secret in it would land
// in cairn's own log. The token is in a header for that reason, and the
// message a failed poll produces has to stay clear of it.
func TestAFailedPollDoesNotPrintTheToken(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte("s3cr3t"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := status.Fetch(status.Source{Provider: "json", URL: srv.URL, TokenFile: file,
		Map: status.Mapping{Key: "n", State: "s"}})
	if err == nil {
		t.Fatal("a 401 was not reported at all")
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("the token is in the message an operator reads in a log: %q", err)
	}
	// And the message still has to be actionable, which for a 401 means saying
	// which key holds the credential that was refused.
	if !strings.Contains(err.Error(), "status.token_file") {
		t.Errorf("a refused credential does not name the key holding it: %q", err)
	}
}

// A file that is not there is a config problem, not a network one, and the
// message says which key names it.
func TestAMissingTokenFileNamesTheKey(t *testing.T) {
	_, err := status.Fetch(status.Source{Provider: "json", URL: "https://example.org",
		TokenFile: "/nowhere/token", Map: status.Mapping{Key: "n", State: "s"}})
	if err == nil {
		t.Fatal("a token file that does not exist was not reported")
	}
	if !strings.Contains(err.Error(), "status.token_file") || !strings.Contains(err.Error(), "/nowhere/token") {
		t.Errorf("message %q names neither the key nor the path", err)
	}
}
