package config

import (
	"testing"
)

func TestLocaleTablesComplete(t *testing.T) {
	want := builtinStrings["en"]
	for lang, table := range builtinStrings {
		if len(table) != len(want) {
			t.Errorf("locale %q has %d strings, en has %d", lang, len(table), len(want))
		}
		for key := range want {
			if table[key] == "" {
				t.Errorf("locale %q is missing %q", lang, key)
			}
		}
	}
}
