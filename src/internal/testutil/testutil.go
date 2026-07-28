// Package testutil holds the one helper every package's tests need: a config
// directory written to a temporary folder. It exists so the helper is not
// copied five times; anything more specific belongs beside the tests using it.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteFiles drops each name/content pair into a fresh temporary directory and
// returns its path, ready to hand to config.Load.
func WriteFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
