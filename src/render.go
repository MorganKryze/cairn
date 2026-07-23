package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"sort"
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
	Skip, Languages, SearchLabel, SearchPlaceholder, SearchEmpty, Open, Back, More, Link, Toc, LinksLabel, Dismiss, Theme, Powered string
}

type pageView struct {
	Locale, SiteTitle, PageTitle, MetaDesc, Logo, Accent, SwitchPath string
	CustomCSS, Search, Credit                                        bool
	Locales                                                          []string
	Links                                                            []linkView
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

type linkView struct {
	Label, URL string
	IconSVG    template.HTML // one of linkGlyphs, trusted markup
	IconIMG    string        // user-supplied URL or /assets path
}

// linkGlyphs are the built-in header-link icons: generic concepts (contact,
// donate, status…) that app-logo collections don't cover. Inline, so a page
// with links still makes zero external requests.
var linkGlyphs = map[string]string{
	"book":      `<path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>`,
	"chat":      `<path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/>`,
	"github":    `<path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.4 5.4 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65S8.93 17.38 9 18v4"/><path d="M9 18c-4.51 2-5-2-7-2"/>`,
	"globe":     `<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20M2 12h20"/>`,
	"heart":     `<path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/>`,
	"home":      `<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`,
	"key":       `<path d="m21 2-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777Zm0 0L15.5 7.5m0 0 3 3L22 7l-3-3m-3.5 3.5L19 4"/>`,
	"mail":      `<rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>`,
	"portfolio": `<rect x="2" y="7" width="20" height="14" rx="2"/><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2"/>`,
	"rss":       `<path d="M4 11a9 9 0 0 1 9 9M4 4a16 16 0 0 1 16 16"/><circle cx="5" cy="19" r="1"/>`,
	"status":    `<path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2"/>`,
	"user":      `<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>`,
}

func linkGlyphNames() []string {
	names := make([]string, 0, len(linkGlyphs))
	for n := range linkGlyphs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func linkIcon(v *linkView, icon string) {
	switch {
	case icon == "":
	case strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") || strings.HasPrefix(icon, "/"):
		v.IconIMG = icon
	default:
		v.IconSVG = template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + linkGlyphs[icon] + `</svg>`)
	}
}

type homeView struct {
	pageView
	Tagline string
	About   []string
	Cats    []catView
}

type detailView struct {
	pageView
	Name, Desc, Icon, URL, Status, StatusLabel, StatusHref string
	Paragraphs                                             []string
}

type staticView struct {
	pageView
	Title      string
	Paragraphs []string
	Sections   []sectionView
}

type sectionView struct {
	Title      string
	Paragraphs []string
}

// statusMeta returns the localized label and the Gatus link for a status, so
// the pill can name itself and send the visitor to its own endpoint page.
func statusMeta(cfg *Config, loc, status string, s Service) (label, href string) {
	if status == "" {
		return "", ""
	}
	// The pill link is for the visitor's browser, so it uses the public
	// status.page URL; status.gatus may be an internal poll-only address.
	base := cfg.Site.Status.Page
	if base == "" {
		base = cfg.Site.Status.Gatus
	}
	href = strings.TrimSuffix(base, "/") + "/endpoints/" + gatusKey(s.Category, s.ID)
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
			Credit:    cfg.Site.Credit == nil || *cfg.Site.Credit,
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
				LinksLabel:        cfg.Str(loc, "nav.links"),
				Dismiss:           cfg.Str(loc, "about.dismiss"),
				Theme:             cfg.Str(loc, "nav.theme"),
				Powered:           cfg.Str(loc, "foot.powered"),
			},
		}
		for _, l := range cfg.Site.Links {
			lv := linkView{Label: l.Label.Get(loc, def), URL: l.URL}
			linkIcon(&lv, l.Icon)
			base.Links = append(base.Links, lv)
		}
		for _, f := range cfg.Site.Footer {
			base.Footer = append(base.Footer, linkView{Label: f.Label.Get(loc, def), URL: f.URL})
		}
		for _, p := range cfg.Site.Pages {
			base.Footer = append(base.Footer, linkView{Label: p.Title.Get(loc, def), URL: "/" + loc + "/" + p.ID + "/"})
		}

		hv := homeView{pageView: base, Tagline: cfg.Site.Tagline.Get(loc, def), About: paragraphs(cfg.Site.About.Get(loc, def))}
		hv.Search = true
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
				card.StatusLabel, card.StatusHref = statusMeta(cfg, loc, card.Status, s)
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
				dv.StatusLabel, dv.StatusHref = statusMeta(cfg, loc, dv.Status, s)
				dv.PageTitle = dv.Name + " · " + cfg.Site.Title
				dv.MetaDesc = dv.Desc
				dv.SwitchPath = s.ID + "/"
				page, err := render("detail.tmpl", dv)
				if err != nil {
					return nil, fmt.Errorf("render /%s/%s/: %w", loc, s.ID, err)
				}
				pages[loc+"/"+s.ID] = page
			}
		}

		for _, p := range cfg.Site.Pages {
			sv := staticView{pageView: base, Title: p.Title.Get(loc, def), Paragraphs: paragraphs(p.Body.Get(loc, def))}
			for _, s := range p.Sections {
				sv.Sections = append(sv.Sections, sectionView{Title: s.Title.Get(loc, def), Paragraphs: paragraphs(s.Body.Get(loc, def))})
			}
			sv.PageTitle = sv.Title + " · " + cfg.Site.Title
			if len(sv.Paragraphs) > 0 {
				sv.MetaDesc = sv.Paragraphs[0]
			} else if len(sv.Sections) > 0 && len(sv.Sections[0].Paragraphs) > 0 {
				sv.MetaDesc = sv.Sections[0].Paragraphs[0]
			}
			sv.SwitchPath = p.ID + "/"
			page, err := render("page.tmpl", sv)
			if err != nil {
				return nil, fmt.Errorf("render /%s/%s/: %w", loc, p.ID, err)
			}
			pages[loc+"/"+p.ID] = page
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
