package check

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// The chart ships an example values file so somebody can install cairn and see
// a real page rather than the getting-started one. That file is config nobody
// runs on the way past, which is exactly how an example rots: a key gets
// renamed, the sample keeps the old spelling, and the first person to try it
// meets an error the docs promised they would not.
//
// So it is loaded here through cairn's own loader, from the same YAML the
// ConfigMap will carry, and put through the same checks a running site gets.
func TestTheChartExampleIsAConfigCairnAccepts(t *testing.T) {
	// The image build runs this suite with only src/ and schema/ copied in, so
	// the chart is genuinely not there and skipping is the honest answer. It
	// still runs on every checkout, which is where CI lints and measures
	// coverage, and on every local run. Only a missing file skips: anything
	// else is a real failure.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "charts", "cairn", "values-example.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("no charts/ in this build context, which is what the image build looks like")
	}
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		Config map[string]string `yaml:"config"`
	}
	if err := yaml.Unmarshal(raw, &values); err != nil {
		t.Fatalf("the example is not YAML the chart could render: %v", err)
	}
	if len(values.Config) == 0 {
		t.Fatal("the example carries no config block, so installing with it shows the getting-started page")
	}

	dir := t.TempDir()
	for name, body := range values.Config {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("the example does not load: %v", err)
	}

	// It is meant to show what a filled page looks like, so it has to be one.
	if n := config.CountServices(cfg); n < 4 {
		t.Errorf("the example lists %d services: too few to show what a page looks like", n)
	}
	if len(cfg.Categories) < 2 {
		t.Errorf("the example has %d categories, and grouping is half of what a directory does", len(cfg.Categories))
	}
	if len(cfg.Site.Locales) < 2 {
		t.Error("the example is single-language, so it shows nothing about the switcher")
	}

	// And it has to be exemplary: a warning here is one every person who
	// copies this file inherits.
	//
	// Two warnings are the right answer here rather than something to silence.
	// The icons are CDN slugs because -emit-icons needs a writable assets dir,
	// which an example install does not have; and there is no site url,
	// because the person running this has no domain yet and a canonical link
	// pointing at one they do not own is worse than none.
	for _, w := range checkWarnings(cfg, dir) {
		if strings.Contains(w, "load from a CDN") || strings.Contains(w, "has no url") {
			continue
		}
		t.Errorf("the example config earns a warning: %s", w)
	}
}
