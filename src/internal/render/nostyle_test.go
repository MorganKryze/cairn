package render

import (
	"bytes"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// The CSP allows the one hashed <style> block and nothing else, so a style
// attribute would be dropped by the browser, silently, on the page that needed
// it. Every page of the fixture with the most shapes on it is read for one.
func TestNoPageCarriesAStyleAttribute(t *testing.T) {
	cfg, err := config.Load("../../../scripts/fixtures/locales")
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Pages) < 20 {
		t.Fatalf("only %d pages built: the fixture no longer has its shapes", len(m.Pages))
	}
	for name, p := range m.Pages {
		if bytes.Contains(p.HTML, []byte(" style=")) {
			t.Errorf("%s carries a style attribute", name)
		}
	}
}
