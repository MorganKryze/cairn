package config_test

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// loadServices builds a config from one services file and the smallest site
// file that parses, and returns the error rather than failing, since half of
// these assert the refusal.
func loadServices(t *testing.T, services string) (*config.Config, error) {
	t.Helper()
	return config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": services,
	}))
}

func TestStateAcceptsEachOfTheFive(t *testing.T) {
	for _, want := range []config.State{
		config.StateSoon, config.StateRetired, config.StateBeta,
		config.StateDeprecated, config.StateNew,
	} {
		cfg, err := loadServices(t, "- {id: a, url: 'https://a.example.org', name: A, state: "+string(want)+"}\n")
		if err != nil {
			t.Fatalf("state: %s was refused: %v", want, err)
		}
		if got := cfg.Categories[0].Services[0].State; got != want {
			t.Errorf("state: %s decoded as %q", want, got)
		}
	}
}

func TestAnUnknownStateNamesTheFive(t *testing.T) {
	_, err := loadServices(t, "- {id: a, url: 'https://a.example.org', name: A, state: sunsetting}\n")
	if err == nil {
		t.Fatal("state: sunsetting was accepted, so any typo becomes a silent no-op")
	}
	for _, want := range []string{"sunsetting", "soon", "retired", "beta", "deprecated", "new"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %s", want, err)
		}
	}
}

func TestADisablingStateMakesTheURLOptional(t *testing.T) {
	for _, s := range []config.State{config.StateSoon, config.StateRetired} {
		if _, err := loadServices(t, "- {id: a, name: A, state: "+string(s)+"}\n"); err != nil {
			t.Errorf("state: %s with no url was refused: %v", s, err)
		}
	}
}

func TestADecorativeStateStillNeedsAURL(t *testing.T) {
	_, err := loadServices(t, "- {id: a, name: A, state: beta}\n")
	if err == nil {
		t.Fatal("a beta service with no url was accepted, so its card links nowhere")
	}
	// the message has to say which states lift the rule, or the reader is left
	// guessing why one of their services parses and another does not
	for _, want := range []string{"soon", "retired"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q lifts it: %s", want, err)
		}
	}
}

func TestAServiceWithNoStateStillNeedsAURL(t *testing.T) {
	if _, err := loadServices(t, "- {id: a, name: A}\n"); err == nil {
		t.Fatal("the exception leaked to every service: a url is required without a state")
	}
}

func TestAURLLeftBesideARetiredStateIsKept(t *testing.T) {
	cfg, err := loadServices(t, "- {id: a, url: 'https://a.example.org', name: A, state: retired}\n")
	if err != nil {
		t.Fatalf("retiring a service must be adding a line, not deleting one: %v", err)
	}
	if got := cfg.Categories[0].Services[0].URL; got != "https://a.example.org" {
		t.Errorf("the address was dropped: %q", got)
	}
}

// The completeness test holds every locale against the English table, so this
// only has to hold that the five keys exist at all: a typo in a key name is
// invisible to a test that compares tables with each other.
func TestTheFiveStatesAreNamedInTheStringsTable(t *testing.T) {
	cfg, err := loadServices(t, "- {id: a, url: 'https://a.example.org', name: A}\n")
	if err != nil {
		t.Fatal(err)
	}
	// Str echoes the key back when it knows no string for it, so "not empty"
	// is not the question: a badge reading "state.soon" is the failure this
	// has to catch, and it passed against the key itself the first time.
	for _, s := range []string{"soon", "retired", "beta", "deprecated", "new"} {
		key := "state." + s
		if got := cfg.Str("en", key); got == "" || got == key {
			t.Errorf("state.%s has no English label: the badge would read %q", s, got)
		}
	}
}
