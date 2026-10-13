package render

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// A card foot sets a hosting word beside a status word, and the browser suite
// measures that pair in every built-in language. It reads the words from
// scripts/fixtures/locales/foot-words.json rather than from a page, because a
// page only ever carries the states its monitor happened to report. This holds
// the file to the strings table, so a word added or reworded here cannot leave
// the browser measuring an old one.
func TestTheFootWordsTheBrowserMeasuresAreTheTablesOwn(t *testing.T) {
	b, err := os.ReadFile(localesFixture(t, "foot-words.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]struct{ Host, Status []string }
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	for _, loc := range config.BuiltinLocales() {
		want := struct{ Host, Status []string }{
			Host: []string{cfg.Str(loc, "host.self"), cfg.Str(loc, "host.external")},
		}
		for _, k := range []string{"up", "down", "degraded", "maintenance", "unknown"} {
			want.Status = append(want.Status, cfg.Str(loc, "status."+k))
		}
		if !reflect.DeepEqual(got[loc], want) {
			t.Errorf("foot-words.json has %s = %+v, the strings table %+v", loc, got[loc], want)
		}
	}
	if len(got) != len(config.BuiltinLocales()) {
		t.Errorf("foot-words.json lists %d languages, cairn ships %d", len(got), len(config.BuiltinLocales()))
	}
}
