package status_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/status"
)

// serving answers every path with the same body, which is what a status API
// that needs no slug looks like from here.
func serving(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func levels(st map[string]status.State) map[string]string {
	out := make(map[string]string, len(st))
	for k, v := range st {
		out[k] = v.Level
	}
	return out
}

// One code path, three vendors, only the mapping differing. That is the whole
// claim of this provider, and the bodies below are the shapes their APIs
// actually answer with, which a spike read live before any of this was
// written.
func TestOneMappingShapeReadsEveryFlatAPI(t *testing.T) {
	for _, c := range []struct {
		name, body string
		m          status.Mapping
		want       map[string]string
	}{
		{"atlassian statuspage, every state it declares",
			`{"components":[{"name":"Packages","status":"operational"},
			                {"name":"Actions","status":"under_maintenance"},
			                {"name":"Pages","status":"degraded_performance"},
			                {"name":"API","status":"partial_outage"},
			                {"name":"Git","status":"major_outage"}]}`,
			status.Mapping{List: "components", Key: "name", State: "status",
				Up:          []string{"operational"},
				Degraded:    []string{"degraded_performance", "partial_outage"},
				Maintenance: []string{"under_maintenance"}},
			map[string]string{"Packages": "up", "Actions": "maintenance",
				"Pages": "degraded", "API": "degraded", "Git": "down"}},
		{"upptime, a bare array with its three states",
			`[{"slug":"google","status":"up"},{"slug":"slow","status":"degraded"},
			  {"slug":"gone","status":"down"}]`,
			status.Mapping{Key: "slug", State: "status",
				Up: []string{"up"}, Degraded: []string{"degraded"}},
			map[string]string{"google": "up", "slow": "degraded", "gone": "down"}},
		{"better stack, nested behind a dotted path",
			`{"data":[{"attributes":{"pronounceable_name":"api","status":"up"}},
			          {"attributes":{"pronounceable_name":"cdn","status":"paused"}}]}`,
			status.Mapping{List: "data", Key: "attributes.pronounceable_name",
				State: "attributes.status", Up: []string{"up"}},
			// paused is in no list, so it reads as down rather than as a state
			// cairn invents a meaning for.
			map[string]string{"api": "up", "cdn": "down"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			st, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, c.body), Map: c.m})
			if err != nil {
				t.Fatal(err)
			}
			got := levels(st)
			if len(got) != len(c.want) {
				t.Fatalf("levels = %v, want %v", got, c.want)
			}
			for k, want := range c.want {
				if got[k] != want {
					t.Errorf("%s = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

// The three lists are allow-lists, checked maintenance, degraded, up, then
// down. A state nobody anticipated therefore reads as down: a vendor adding a
// word next year must not make a broken service look green, and an operator
// who forgot to list one gets a red pill rather than a wrong one.
func TestAnUnknownStateReadsAsDown(t *testing.T) {
	body := `[{"n":"a","s":"operational"},{"n":"b","s":"sunny"},{"n":"c","s":"OPERATIONAL"}]`
	m := status.Mapping{Key: "n", State: "s", Up: []string{"operational"}}
	st, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, body), Map: m})
	if err != nil {
		t.Fatal(err)
	}
	if st["a"].Level != status.LevelUp {
		t.Errorf("a listed state read as %q", st["a"].Level)
	}
	for _, name := range []string{"b", "c"} {
		if st[name].Level != status.LevelDown {
			t.Errorf("%s read as %q, want down: a value not in a list is not up", name, st[name].Level)
		}
	}
}

// A monitor whose checking is switched off has said nothing about the service,
// so it gets no pill rather than a red one. Found live: on a real UptimeRobot
// status page, of 15 monitors several read statusClass "paused", and every one
// of them was drawn as down, which tells a visitor a working service is broken.
// Kuma's own pending is skipped for the same reason, and this is the mapper's
// way of saying it for a vocabulary cairn cannot know in advance.
func TestAStateInTheUnknownListDrawsNoPill(t *testing.T) {
	body := `[{"n":"live","s":"success"},{"n":"asleep","s":"paused"},{"n":"broken","s":"danger"}]`
	m := status.Mapping{Key: "n", State: "s", Up: []string{"success"}, Unknown: []string{"paused"}}
	st, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, body), Map: m})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st["asleep"]; ok {
		t.Errorf("a paused monitor was given a pill: %q", st["asleep"].Level)
	}
	if st["live"].Level != status.LevelUp || st["broken"].Level != status.LevelDown {
		t.Errorf("levels = %v, want live up and broken down", levels(st))
	}
	// Unknown is checked before the rest, so a value listed twice by mistake
	// resolves to the quiet answer rather than to whichever list came first.
	both := status.Mapping{Key: "n", State: "s", Up: []string{"paused"}, Unknown: []string{"paused"}}
	st, err = status.Fetch(status.Source{Provider: "json", URL: serving(t, body), Map: both})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st["asleep"]; ok {
		t.Error("a value in both lists was painted rather than left quiet")
	}
}

// The document belongs to somebody else and will not always be the shape the
// mapping claims. Each message says what was looked for and what was actually
// there, because -check makes no network request and cannot say it first.
func TestAMappingThatFindsNothingSaysWhatWasThere(t *testing.T) {
	for _, c := range []struct {
		name, body string
		m          status.Mapping
		wants      []string
	}{
		{"a list path that is a typo",
			`{"components":[{"name":"a","status":"operational"}],"page":{"id":"x"}}`,
			status.Mapping{List: "compnents", Key: "name", State: "status"},
			[]string{"status.map.list", "compnents", "components", "page"}},
		{"a list path that reaches something other than an array",
			`{"components":{"name":"a"}}`,
			status.Mapping{List: "components", Key: "name", State: "status"},
			[]string{"status.map.list", "components", "array"}},
		{"a key path no row has",
			`{"components":[{"name":"a","status":"operational"}]}`,
			status.Mapping{List: "components", Key: "nom", State: "status"},
			[]string{"status.map.key", "nom", "name", "status"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, c.body), Map: c.m})
			if err == nil {
				t.Fatal("a mapping that read nothing reported no problem")
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q does not carry %q", err, want)
				}
			}
		})
	}
}

// A row the mapping cannot read is skipped, not fatal. Real status pages carry
// group headers and marketing rows: GitHub's own has a component named "Visit
// www.githubstatus.com for more information". cairn only draws a pill for a
// name matching a service id, so junk is already harmless, and one bad row
// must not cost the whole poll.
func TestARowTheMappingCannotReadIsSkipped(t *testing.T) {
	body := `{"components":[{"name":"a","status":"operational"},
	                        {"name":"header"},
	                        {"status":"operational"},
	                        "a string where an object was expected",
	                        {"name":"b","status":"major_outage"}]}`
	m := status.Mapping{List: "components", Key: "name", State: "status", Up: []string{"operational"}}
	st, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, body), Map: m})
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 || st["a"].Level != status.LevelUp || st["b"].Level != status.LevelDown {
		t.Errorf("levels = %v, want the two readable rows and nothing else", levels(st))
	}
}

// An empty list is not an error. A status page with nothing on it yet answers
// exactly like this, and every pill reading Unknown is the honest result.
func TestAnEmptyListIsNotAFailure(t *testing.T) {
	st, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, `{"components":[]}`),
		Map: status.Mapping{List: "components", Key: "name", State: "status"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 0 {
		t.Errorf("an empty list produced %v", levels(st))
	}
}

// A page whose every monitor is paused read fine: the mapping found names and
// states, it is just that none of them says anything. Counting what was drawn
// rather than what was found would report that as a broken mapping and log an
// error at every poll.
func TestAPageThatIsEntirelyQuietIsNotAFailure(t *testing.T) {
	body := `[{"n":"a","s":"paused"},{"n":"b","s":"paused"}]`
	m := status.Mapping{Key: "n", State: "s", Up: []string{"success"}, Unknown: []string{"paused"}}
	st, err := status.Fetch(status.Source{Provider: "json", URL: serving(t, body), Map: m})
	if err != nil {
		t.Fatalf("a page where nothing is being checked was reported as a broken mapping: %v", err)
	}
	if len(st) != 0 {
		t.Errorf("levels = %v, want nothing at all", levels(st))
	}
}
