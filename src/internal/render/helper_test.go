package render

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// modelFrom builds the model a config directory would be served as, which is
// what most render tests assert against.
func modelFrom(t *testing.T, files map[string]string) *Model {
	t.Helper()
	cfg, err := config.Load(testutil.WriteFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// sample loads a small two-service config, enough to render or poll against.
func sample(t *testing.T) *config.Config {
	t.Helper()
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nstatus: {gatus: https://status.example.org}\n",
		"services.yaml": "- {id: pdf, url: https://pdf.example.org, category: documents, name: PDF}\n" +
			"- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// localesFixture is the path of a file under scripts/fixtures/locales, the
// fixture the browser suite serves. The image build runs this suite with only
// src/ and schema/ copied in, so there the fixture is genuinely absent and the
// test skips; every checkout, CI's included, still runs it. Only a missing
// file skips: anything else is a real failure.
func localesFixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "scripts", "fixtures", "locales", name)
	if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
		t.Skip("no scripts/fixtures in this build context, which is what the image build looks like")
	} else if err != nil {
		t.Fatal(err)
	}
	return p
}
