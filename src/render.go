package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates assets
var embedded embed.FS

var homeTmpl = template.Must(template.New("home.tmpl").Funcs(template.FuncMap{
	"upper": strings.ToUpper,
}).ParseFS(embedded, "templates/home.tmpl"))

type Page struct {
	HTML []byte
	ETag string
}

// Model is the immutable unit swapped atomically on config reload.
type Model struct {
	Cfg   *Config
	Pages map[string]Page
}

type uiStrings struct {
	Skip, Languages, SearchLabel, SearchPlaceholder, SearchEmpty string
}

type cardView struct {
	URL, Icon, Name, Desc, Tags string
}

type catView struct {
	ID, Name string
	Cards    []cardView
}

type linkView struct{ Label, URL string }

type homeView struct {
	Locale, Title, Tagline, Logo, Accent string
	CustomCSS                            bool
	Locales                              []string
	Cats                                 []catView
	Footer                               []linkView
	S                                    uiStrings
}

func buildModel(cfg *Config) (*Model, error) {
	def := cfg.DefaultLocale()
	pages := make(map[string]Page, len(cfg.Site.Locales))
	for _, loc := range cfg.Site.Locales {
		v := homeView{
			Locale:    loc,
			Title:     cfg.Site.Title,
			Tagline:   cfg.Site.Tagline.Get(loc, def),
			Logo:      cfg.Site.Logo,
			Accent:    cfg.Site.Theme.Accent,
			CustomCSS: cfg.CustomCSS,
			Locales:   cfg.Site.Locales,
			S: uiStrings{
				Skip:              cfg.Str(loc, "nav.skip"),
				Languages:         cfg.Str(loc, "nav.languages"),
				SearchLabel:       cfg.Str(loc, "search.label"),
				SearchPlaceholder: cfg.Str(loc, "search.placeholder"),
				SearchEmpty:       cfg.Str(loc, "search.empty"),
			},
		}
		for _, c := range cfg.Categories {
			cv := catView{ID: c.ID, Name: cfg.categoryName(c, loc)}
			for _, s := range c.Services {
				cv.Cards = append(cv.Cards, cardView{
					URL:  s.URL,
					Icon: iconURL(s.Icon),
					Name: s.Name.Get(loc, def),
					Desc: s.Desc.Get(loc, def),
					Tags: strings.Join(s.Tags, " "),
				})
			}
			v.Cats = append(v.Cats, cv)
		}
		for _, f := range cfg.Site.Footer {
			v.Footer = append(v.Footer, linkView{Label: f.Label.Get(loc, def), URL: f.URL})
		}
		var buf bytes.Buffer
		if err := homeTmpl.Execute(&buf, v); err != nil {
			return nil, fmt.Errorf("render %s: %w", loc, err)
		}
		sum := sha256.Sum256(buf.Bytes())
		pages[loc] = Page{HTML: buf.Bytes(), ETag: fmt.Sprintf("%q", hex.EncodeToString(sum[:8]))}
	}
	return &Model{Cfg: cfg, Pages: pages}, nil
}

// iconURL resolves a bare slug against dashboard-icons, the convention
// Homepage and Homarr already use; URLs and absolute paths pass through.
func iconURL(icon string) string {
	if icon == "" || strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") || strings.HasPrefix(icon, "/") {
		return icon
	}
	return "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/" + icon + ".svg"
}
