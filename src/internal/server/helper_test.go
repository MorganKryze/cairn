package server

import (
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

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
