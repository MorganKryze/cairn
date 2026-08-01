package status_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/status"
)

// A Source that names no provider is the one every config produces today, and
// it has to keep meaning Gatus. The zero value is the shape a future caller
// will reach for by accident, so it is the one worth pinning.
func TestAnUnnamedProviderIsGatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/endpoints/statuses" {
			t.Errorf("an unnamed provider asked for %q, which is not the gatus path", r.URL.Path)
		}
		io.WriteString(w, `[{"name":"pad","key":"_pad","results":[{"success":true}]}]`)
	}))
	defer srv.Close()

	st, err := status.Fetch(status.Source{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if st["pad"].Level != status.LevelUp {
		t.Errorf("pad = %q, want %q", st["pad"].Level, status.LevelUp)
	}
	if st["pad"].Key != "_pad" {
		t.Errorf("the reported key was dropped: %q", st["pad"].Key)
	}
}

// A provider nobody wrote is an error naming the ones that exist, not a silent
// fall back to Gatus against an address that is not one. The list comes from
// the registry, so a provider added later is one the message already knows
// about; until then gatus is the whole list, and this test says so.
func TestAnUnknownProviderSaysWhichExist(t *testing.T) {
	_, err := status.Fetch(status.Source{URL: "https://example.org", Provider: "nagios"})
	if err == nil {
		t.Fatal("an unknown provider was accepted")
	}
	for _, want := range []string{"nagios", "gatus"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}
