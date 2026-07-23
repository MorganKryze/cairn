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

var tmpls = template.Must(template.New("").Funcs(template.FuncMap{
	"upper": strings.ToUpper,
}).ParseFS(embedded, "templates/*.tmpl"))

type Page struct {
	HTML []byte
	ETag string
}

// Model is the immutable unit swapped atomically on config reload or status
// change. Pages is keyed by URL path without slashes: "fr" for a home,
// "fr/pdf" for a detail. Statuses is service-id -> up, from Gatus.
type Model struct {
	Cfg      *Config
	Pages    map[string]Page
	Statuses map[string]bool
}

type uiStrings struct {
	Skip, Languages, SearchLabel, SearchPlaceholder, SearchEmpty, Open, Back, More, Link, Toc string
}

type pageView struct {
	Locale, SiteTitle, PageTitle, MetaDesc, Logo, Accent, SwitchPath string
	CustomCSS                                                        bool
	Locales                                                          []string
	Footer                                                           []linkView
	S                                                                uiStrings
}

type cardView struct {
	URL, Icon, Name, Desc, Tags, MoreHref, Status, StatusLabel, StatusHref string
}

type catView struct {
	ID, Name string
	Cards    []cardView
}

type linkView struct{ Label, URL string }

type homeView struct {
	pageView
	Tagline string
	Cats    []catView
}

type detailView struct {
	pageView
	Name, Desc, Icon, URL, Status, StatusLabel, StatusHref string
	Paragraphs                                             []string
}

// statusMeta returns the localized label and the Gatus link for a status, so
// the pill can name itself and send the visitor to the status page.
func statusMeta(cfg *Config, loc, status string) (label, href string) {
	if status == "" {
		return "", ""
	}
	// The pill link is for the visitor's browser, so it uses the public
	// status.page URL — status.gatus may be an internal poll-only address.
	href = cfg.Site.Status.Page
	if href == "" {
		href = cfg.Site.Status.Gatus
	}
	return cfg.Str(loc, "status."+status), href
}

// statusOf returns "", "unknown", "up" or "down". While Gatus has not
// answered yet (boot, outage) every pill is unknown; once it has, services it
// does not monitor show no pill at all.
func statusOf(cfg *Config, statuses map[string]bool, id string) string {
	if cfg.Site.Status.Gatus == "" {
		return ""
	}
	if len(statuses) == 0 {
		return "unknown"
	}
	up, ok := statuses[id]
	switch {
	case !ok:
		return ""
	case up:
		return "up"
	default:
		return "down"
	}
}

func buildModel(cfg *Config, statuses map[string]bool) (*Model, error) {
	def := cfg.DefaultLocale()
	pages := map[string]Page{}
	for _, loc := range cfg.Site.Locales {
		base := pageView{
			Locale:    loc,
			SiteTitle: cfg.Site.Title,
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
				Open:              cfg.Str(loc, "detail.open"),
				Back:              cfg.Str(loc, "detail.back"),
				More:              cfg.Str(loc, "card.more"),
				Link:              cfg.Str(loc, "status.link"),
				Toc:               cfg.Str(loc, "nav.toc"),
			},
		}
		for _, f := range cfg.Site.Footer {
			base.Footer = append(base.Footer, linkView{Label: f.Label.Get(loc, def), URL: f.URL})
		}

		hv := homeView{pageView: base, Tagline: cfg.Site.Tagline.Get(loc, def)}
		hv.PageTitle = cfg.Site.Title
		hv.MetaDesc = hv.Tagline
		for _, c := range cfg.Categories {
			cv := catView{ID: c.ID, Name: cfg.categoryName(c, loc)}
			for _, s := range c.Services {
				card := cardView{
					URL:    s.URL,
					Icon:   iconURL(s.Icon),
					Name:   s.Name.Get(loc, def),
					Desc:   s.Desc.Get(loc, def),
					Tags:   strings.Join(s.Tags, " "),
					Status: statusOf(cfg, statuses, s.ID),
				}
				card.StatusLabel, card.StatusHref = statusMeta(cfg, loc, card.Status)
				if len(s.Details) > 0 {
					card.MoreHref = "/" + loc + "/" + s.ID + "/"
				}
				cv.Cards = append(cv.Cards, card)
			}
			hv.Cats = append(hv.Cats, cv)
		}
		page, err := render("home.tmpl", hv)
		if err != nil {
			return nil, fmt.Errorf("render /%s/: %w", loc, err)
		}
		pages[loc] = page

		for _, c := range cfg.Categories {
			for _, s := range c.Services {
				dv := detailView{
					pageView:   base,
					Name:       s.Name.Get(loc, def),
					Desc:       s.Desc.Get(loc, def),
					Icon:       iconURL(s.Icon),
					URL:        s.URL,
					Status:     statusOf(cfg, statuses, s.ID),
					Paragraphs: paragraphs(s.Details.Get(loc, def)),
				}
				dv.StatusLabel, dv.StatusHref = statusMeta(cfg, loc, dv.Status)
				dv.PageTitle = dv.Name + " — " + cfg.Site.Title
				dv.MetaDesc = dv.Desc
				dv.SwitchPath = s.ID + "/"
				page, err := render("detail.tmpl", dv)
				if err != nil {
					return nil, fmt.Errorf("render /%s/%s/: %w", loc, s.ID, err)
				}
				pages[loc+"/"+s.ID] = page
			}
		}
	}
	return &Model{Cfg: cfg, Pages: pages, Statuses: statuses}, nil
}

func render(name string, v any) (Page, error) {
	var buf bytes.Buffer
	if err := tmpls.ExecuteTemplate(&buf, name, v); err != nil {
		return Page{}, err
	}
	sum := sha256.Sum256(buf.Bytes())
	return Page{HTML: buf.Bytes(), ETag: fmt.Sprintf("%q", hex.EncodeToString(sum[:8]))}, nil
}

func paragraphs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// iconURL resolves a bare slug against dashboard-icons, the convention
// Homepage and Homarr already use; URLs and absolute paths pass through.
func iconURL(icon string) string {
	if icon == "" || strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") || strings.HasPrefix(icon, "/") {
		return icon
	}
	return "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/" + icon + ".svg"
}
