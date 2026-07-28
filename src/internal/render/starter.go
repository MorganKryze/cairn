package render

import (
	"log"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// The getting-started page: what cairn serves instead of dying when the
// config directory is empty or invalid at boot. It goes through the normal
// rendering pipeline, so it looks like the product and needs nothing special.
func StarterModel() *Model {
	cfg := &config.Config{Site: config.Site{Title: config.LString{"": "cairn"}, Locales: []string{"en"}}}
	cfg.Site.Theme.Accent = "#247b7b"
	cfg.Site.About = config.LString{"": config.StarterAbout}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		// Templates are Embedded; this cannot fail outside development.
		log.Fatal(err)
	}
	m.Ready = false // this page is a stand-in, not the operator's site
	return m
}
