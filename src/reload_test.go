package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reload path is where a bad edit could take a live site down, so the
// contract is: a valid change swaps in, a broken one keeps the old pages.
func TestReloadOnce(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml":     "title: First\nlocales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
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
	print1 := watchPrint(dir)

	// nothing touched: the fingerprint holds and the pages are left alone
	if got := reloadOnce(dir, print1); got != print1 {
		t.Error("an untouched directory reported a change")
	}

	// a real edit lands
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("site.yaml", "title: Second\nlocales: [en]\n")
	print2 := reloadOnce(dir, print1)
	if print2 == print1 {
		t.Fatal("an edited directory kept the old fingerprint")
	}
	if got := string(current.Load().Pages["en"].HTML); !strings.Contains(got, "Second") {
		t.Error("the edit did not reach the served page")
	}

	// a broken edit is refused, and the previous pages stay up
	write("services.yaml", "- {id: pad, name: no url here}\n")
	print3 := reloadOnce(dir, print2)
	if print3 == print2 {
		t.Error("a broken edit should still advance the fingerprint, or it retries forever")
	}
	if got := string(current.Load().Pages["en"].HTML); !strings.Contains(got, "Second") {
		t.Error("a broken config took the live pages down")
	}
}

// Statuses come from Gatus; the dots must disappear rather than go stale when
// it stops answering, and a poll must not clobber a config reload.
func TestPollOnce(t *testing.T) {
	up := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if up {
			w.Write([]byte(`[{"name":"pad","results":[{"success":true}]}]`))
			return
		}
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()

	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\nstatus: {gatus: " + srv.URL + "}\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	var seen pollLog

	// no gatus configured is a no-op, not a crash
	before := current.Load()
	pollOnce("", &seen)
	if current.Load() != before {
		t.Error("an empty gatus url rebuilt the model")
	}

	pollOnce(srv.URL, &seen)
	if got := current.Load().Statuses["pad"]; !got {
		t.Fatalf("statuses = %v, want pad up", current.Load().Statuses)
	}
	if !strings.Contains(string(current.Load().Pages["en"].HTML), "status-up") {
		t.Error("the green pill never reached the page")
	}

	// same answer twice: no needless rebuild
	same := current.Load()
	pollOnce(srv.URL, &seen)
	if current.Load() != same {
		t.Error("an unchanged status rebuilt the pages")
	}

	// gatus falls over: the dots go away instead of lying
	up = false
	pollOnce(srv.URL, &seen)
	if n := len(current.Load().Statuses); n != 0 {
		t.Errorf("statuses = %d, want none once gatus stops answering", n)
	}
	if strings.Contains(string(current.Load().Pages["en"].HTML), "status-up") {
		t.Error("a stale green pill survived the outage")
	}
}

// -check is the command operators wire into their own CI, so its exit code is
// part of the contract: 0 for a config that would serve, 1 for one that would
// not.
func TestRunCheckExitCodes(t *testing.T) {
	good := writeFiles(t, map[string]string{
		"site.yaml":     "locales: [en, fr]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	if code := runCheck(good); code != 0 {
		t.Errorf("a valid config exited %d, want 0", code)
	}

	bad := writeFiles(t, map[string]string{
		"services.yaml": "- {id: pad, name: no url at all}\n",
	})
	if code := runCheck(bad); code != 1 {
		t.Errorf("an invalid config exited %d, want 1", code)
	}

	if code := runCheck(filepath.Join(good, "does-not-exist")); code != 1 {
		t.Errorf("a missing directory exited %d, want 1", code)
	}
}

// The container healthcheck is this function; it must answer 0 only when a
// server is really listening on the port it was given.
func TestProbe(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if code := probe(":" + port); code != 0 {
		t.Errorf("probe against a live server exited %d, want 0", code)
	}
	if code := probe(":1"); code != 0 && code != 1 {
		t.Errorf("unexpected exit %d", code)
	}
	if code := probe("not-an-address"); code != 1 {
		t.Errorf("probe on a malformed address exited %d, want 1", code)
	}
}
