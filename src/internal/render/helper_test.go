package render

import (
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
