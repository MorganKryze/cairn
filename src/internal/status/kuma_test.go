package status_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/status"
)

// The two documents a published Kuma status page answers with, recorded from a
// real instance during the audit. Nothing here is invented: the shapes, the
// numeric keys and the status constants are what it sent.
const (
	kumaPage = `{"config":{"slug":"probe","title":"Probe"},"publicGroupList":[
		{"id":1,"name":"Services","monitorList":[
			{"id":1,"name":"pdf","type":"http"},
			{"id":2,"name":"pad","type":"http"}]}]}`
	kumaBeats = `{"heartbeatList":{
		"1":[{"status":1,"time":"2026-07-31 20:27:38.333","ping":142}],
		"2":[{"status":0,"time":"2026-07-31 20:27:39.051"}]},
		"uptimeList":{"1_24":1,"2_24":0}}`
)

// kuma serves one status page and its heartbeats, and fails any other path so
// a provider asking for something else is caught rather than left to 404.
func kuma(t *testing.T, page, beats string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status-page/probe":
			io.WriteString(w, page)
		case "/api/status-page/heartbeat/probe":
			io.WriteString(w, beats)
		default:
			t.Errorf("the kuma provider asked for %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Kuma keys its heartbeats by a numeric monitor id and puts the names in
// another document. The join is the whole provider, and it is why a mapping
// could never express this one.
func TestKumaJoinsNamesToHeartbeats(t *testing.T) {
	srv := kuma(t, kumaPage, kumaBeats)
	st, err := status.Fetch(status.Source{Provider: "kuma", URL: srv.URL, Slug: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	if st["pdf"].Level != status.LevelUp || st["pad"].Level != status.LevelDown {
		t.Errorf("statuses = %v, want pdf up and pad down", st)
	}
	// Kuma has no per-monitor page, so there is no key to report and the pill
	// falls back to the one status page every service shares.
	if st["pdf"].Key != "" {
		t.Errorf("pdf claimed the key %q; kuma has no per-monitor page", st["pdf"].Key)
	}
}

// Kuma's four constants, read from its own source: 0 down, 1 up, 2 pending,
// 3 maintenance. Pending is left out of the map rather than given a level of
// its own: a monitor nobody has checked yet is what cairn has painted Unknown
// since the beginning, and a second way of saying that would be two pills for
// one idea.
//
// There is deliberately no degraded here. Four sources of eight distinguish
// one and Kuma is not among them, so inventing one would be cairn claiming a
// monitor said something it did not.
func TestKumaStates(t *testing.T) {
	page := `{"publicGroupList":[{"monitorList":[
		{"id":1,"name":"up"},{"id":2,"name":"down"},
		{"id":3,"name":"pending"},{"id":4,"name":"maintenance"}]}]}`
	beats := `{"heartbeatList":{
		"1":[{"status":1}],"2":[{"status":0}],
		"3":[{"status":2}],"4":[{"status":3}]}}`
	st, err := status.Fetch(status.Source{Provider: "kuma", URL: kuma(t, page, beats).URL, Slug: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"up": status.LevelUp, "down": status.LevelDown, "maintenance": status.LevelMaintenance,
	} {
		if st[name].Level != want {
			t.Errorf("%s = %q, want %q", name, st[name].Level, want)
		}
	}
	if _, ok := st["pending"]; ok {
		t.Errorf("pending was given a pill: %q", st["pending"].Level)
	}
	// The last beat is the current one, the way the last Gatus result is.
	last := `{"heartbeatList":{"1":[{"status":1},{"status":0}],"2":[{"status":0}],"3":[{"status":2}],"4":[{"status":3}]}}`
	st, err = status.Fetch(status.Source{Provider: "kuma", URL: kuma(t, page, last).URL, Slug: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	if st["up"].Level != status.LevelDown {
		t.Errorf("a monitor whose latest beat failed reads %q", st["up"].Level)
	}
}

// A monitor with no heartbeat at all is absent rather than down. Guessing down
// would paint a red pill for a service nobody has looked at yet, which is the
// one thing a status pill must never do.
func TestKumaSkipsAMonitorWithNoHeartbeat(t *testing.T) {
	page := `{"publicGroupList":[{"monitorList":[{"id":1,"name":"pdf"},{"id":9,"name":"fresh"}]}]}`
	st, err := status.Fetch(status.Source{Provider: "kuma", URL: kuma(t, page, `{"heartbeatList":{"1":[{"status":1}],"9":[]}}`).URL, Slug: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st["fresh"]; ok {
		t.Errorf("a monitor with no heartbeat was given a pill: %q", st["fresh"].Level)
	}
	if st["pdf"].Level != status.LevelUp {
		t.Error("the monitor that did have a heartbeat was lost with it")
	}
}

// Every failure an operator will actually hit, each naming the key to fix.
// cairn logs one line per poll and stays quiet after that, so that line is the
// whole diagnosis.
func TestKumaFailuresNameTheKey(t *testing.T) {
	for _, c := range []struct {
		name  string
		serve http.HandlerFunc
		slug  string
		wants []string
	}{
		// The commonest of the three by far: a status page exists in the admin
		// but was never published, and the endpoint 404s exactly as if the slug
		// were misspelled.
		{"a slug that is not published", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}, "probe", []string{"status.slug", "probe", "published"}},
		{"a status page with no monitors attached", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"publicGroupList":[]}`)
		}, "probe", []string{"status.slug", "probe", "no monitor"}},
		{"a body that is not the document it claims", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "<!doctype html><title>Uptime Kuma</title>")
		}, "probe", []string{"status.url", "kuma"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(c.serve)
			defer srv.Close()
			_, err := status.Fetch(status.Source{Provider: "kuma", URL: srv.URL, Slug: c.slug})
			if err == nil {
				t.Fatal("the poll reported no problem at all")
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q does not carry %q", err, want)
				}
			}
		})
	}
}
