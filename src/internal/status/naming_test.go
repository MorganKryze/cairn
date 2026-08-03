package status_test

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// Reported on issue #33, from a working Uptime Kuma: the monitor was named
// "Immich", the way a person names a thing in a form, and the service id was
// immich, because cairn forces an id to lowercase. Nothing matched and the
// card simply had no pill.
//
// A service id can only be lowercase, so folding what a monitor reports can
// add a match and can never invent a wrong one. It is done for every provider
// rather than for Kuma alone: Statuspage, Cachet and Upptime name their
// components by hand too, and Gatus is unaffected because it reports a key it
// has already normalised itself.
func TestAMonitorNameMatchesWhateverItsCase(t *testing.T) {
	page := `{"publicGroupList":[{"id":1,"name":"Services","monitorList":[
		{"id":1,"name":"Immich","type":"http"},
		{"id":2,"name":"PDF","type":"http"}]}]}`
	beats := `{"heartbeatList":{"1":[{"status":1}],"2":[{"status":0}]}}`
	srv := kuma(t, page, beats)

	got, err := status.Fetch(status.Source{URL: srv.URL, Provider: "kuma", Slug: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"immich": status.LevelUp, "pdf": status.LevelDown} {
		st, ok := got[id]
		if !ok {
			t.Errorf("the service id %q finds nothing, so its card shows no pill; keys came back as %v", id, got)
			continue
		}
		if st.Level != want {
			t.Errorf("%s is %q, want %q", id, st.Level, want)
		}
	}
}

// The same for the json mapper, whose key is whatever field an operator
// pointed at, and which is written by a person just as often.
func TestAMappedKeyMatchesWhateverItsCase(t *testing.T) {
	body := `{"components":[{"name":"Photo-Backup","status":"operational"}]}`
	got, err := status.Fetch(status.Source{
		URL: serving(t, body), Provider: "json",
		Map: status.Mapping{List: "components", Key: "name", State: "status", Up: []string{"operational"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["photo-backup"]; !ok {
		t.Errorf("a mapped key keeps its case: %v", got)
	}
}

// Folding the case closes the commonest mismatch, not every one: a monitor
// named after a domain, or with a space in it, still matches nothing. The log
// line is then the only thing standing between an operator and a silent
// missing pill, and listing the ids it did not find is half an answer. What
// they need is the other half, the names the monitor did offer, because the
// mismatch is obvious the moment the two lists sit side by side.
func TestTheUnmonitoredLineSaysWhatTheMonitorOffered(t *testing.T) {
	cfg := twoServiceConfig(t)
	line := status.Unmonitored(cfg, map[string]status.State{
		"photo backup":    {Level: status.LevelUp},
		"pad.example.org": {Level: status.LevelUp},
	})
	for _, want := range []string{"pad", "immich", "photo backup", "pad.example.org"} {
		if !strings.Contains(line, want) {
			t.Errorf("the line does not carry %q:\n%s", want, line)
		}
	}
}

// And it stays quiet about the other side when every name was claimed: a
// second list that is always there is a second list nobody reads.
func TestTheUnmonitoredLineOffersNothingWhenEveryNameWasUsed(t *testing.T) {
	cfg := twoServiceConfig(t)
	line := status.Unmonitored(cfg, map[string]status.State{"pad": {Level: status.LevelUp}})
	if strings.Contains(line, "reports") {
		t.Errorf("nothing was left over, so nothing should be offered:\n%s", line)
	}
	if !strings.Contains(line, "immich") {
		t.Errorf("the service with no status went unnamed:\n%s", line)
	}
}

// twoServiceConfig is a config with the two ids the tests above look for.
func twoServiceConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [en]\nstatus:\n  provider: kuma\n  url: https://kuma.example.org\n  slug: tools\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n" +
			"- {id: immich, url: https://immich.example.org, name: Immich}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// One is one. The line is read by an operator at the moment something is
// wrong, and "1 services ... their cards" is a line that reads as though
// nobody looked at it.
func TestTheUnmonitoredLineCountsInWords(t *testing.T) {
	cfg := twoServiceConfig(t)
	one := status.Unmonitored(cfg, map[string]status.State{
		"pad": {Level: status.LevelUp}, "photo backup": {Level: status.LevelUp},
	})
	for _, want := range []string{"1 service the", "its card shows no pill", "1 name no service id matches"} {
		if !strings.Contains(one, want) {
			t.Errorf("singular line missing %q:\n%s", want, one)
		}
	}
	many := status.Unmonitored(cfg, map[string]status.State{"a": {}, "b": {}})
	for _, want := range []string{"2 services the", "their cards show no pill", "2 names no service id matches"} {
		if !strings.Contains(many, want) {
			t.Errorf("plural line missing %q:\n%s", want, many)
		}
	}
}
