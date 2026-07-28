package render

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
)

//go:embed templates assets
var Embedded embed.FS

var tmpls = template.Must(template.New("").Funcs(template.FuncMap{
	"upper": strings.ToUpper,
}).ParseFS(Embedded, "templates/*.tmpl"))

type Page struct {
	HTML []byte
	ETag string
}

// Model is the immutable unit swapped atomically on config reload or status
// change. Pages is keyed by URL path without slashes: "fr" for a home,
// "fr/pdf" for a detail. Statuses is service-id -> up, from Gatus.
type Model struct {
	Cfg      *config.Config
	Pages    map[string]Page
	Statuses map[string]bool
	CSP      string
	// Ready is false only while the getting-started page stands in for a
	// config that never loaded. /readyz reports it; /healthz does not, so a
	// liveness probe cannot restart-loop a container that is serving fine.
	Ready bool
}

// prePaintScript must stay byte-identical to the inline script in
// layout.tmpl: its hash is what the CSP allows. A test guards the match.
// The about cookie stores a hash of the note's content (the data-about
// attribute), so an edited note reappears even for visitors who dismissed
// the previous one.
const prePaintScript = `var a=document.cookie.match(/(?:^|; )about=([^;]*)/);if(a&&a[1]===document.documentElement.getAttribute('data-about'))document.documentElement.setAttribute('data-noabout','');try{var t=localStorage.getItem('theme');if(t)document.documentElement.setAttribute('data-theme',t)}catch(e){}`

// aboutHash fingerprints the welcome note across all locales; eight hex
// chars are plenty for a cookie value that only needs to change when the
// note does.
func aboutHash(about config.LString) string {
	if len(about) == 0 {
		return ""
	}
	keys := make([]string, 0, len(about))
	for k := range about {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k + "\x00" + about[k] + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}

func accentStyle(accent string) string { return ":root{--accent:" + accent + "}" }

func cspHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// BuildCSP allows exactly what the pages use: self assets, the two known
// inline fragments by hash, and images from anywhere https (icon slugs).
func BuildCSP(cfg *config.Config) string {
	return "default-src 'none'; img-src 'self' https: data:; " +
		"style-src 'self' " + cspHash(accentStyle(cfg.Site.Theme.Accent)) + "; " +
		"script-src 'self' " + cspHash(prePaintScript) + "; " +
		"font-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
}

type uiStrings struct {
	Skip, Languages, SearchLabel, SearchPlaceholder, SearchEmpty, SearchOne, SearchMany, Open, Back, More, Link, Toc, LinksLabel, Dismiss, Theme, Powered, Menu, Top string
}

type pageView struct {
	Locale, SiteTitle, PageTitle, MetaDesc, Logo, Favicon, TouchIcon, OGImage, Accent, SwitchPath, Base, Version, AboutHash string
	Prefix                                                                                                                  string // "" or "/cairn", see BasePath
	Dir                                                                                                                     string // "ltr" or "rtl", from the locale
	CustomCSS, Search, Credit, Noindex, ShowVer                                                                             bool
	VerLabel, VerHref                                                                                                       string
	Locales                                                                                                                 []string
	Links                                                                                                                   []linkView
	Footer                                                                                                                  []linkView
	S                                                                                                                       uiStrings
}

type cardView struct {
	URL, Icon, Name, Desc, Tags, MoreHref, Status, StatusLabel, StatusHref string
	HostKind, HostLabel                                                    string // "self"/"external"/"" and its localized label
}

type catView struct {
	ID, Name string
	Cards    []cardView
}

type linkView struct {
	Label, URL string
	IconSVG    template.HTML // one of config.Glyphs, trusted markup
	IconIMG    string        // user-supplied URL or /assets path
}

func linkIcon(v *linkView, icon string) {
	switch {
	case icon == "":
	case config.IsURLOrAbs(icon):
		v.IconIMG = AppURL(icon)
	default:
		v.IconSVG = template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + config.Glyphs[icon] + `</svg>`)
	}
}

type homeView struct {
	pageView
	Tagline string
	About   []template.HTML
	Cats    []catView
}

type detailView struct {
	pageView
	Name, Desc, Icon, URL, Status, StatusLabel, StatusHref string
	Body                                                   []template.HTML
	Images                                                 []imageView
}

type imageView struct {
	Src, Caption string
	W, H         int
}

type staticView struct {
	pageView
	Title    string
	Intro    []template.HTML
	Sections []sectionView
}

type sectionView struct {
	Title string
	Body  []template.HTML
}

// statusMeta returns the localized label and the Gatus link for a status, so
// the pill can name itself and send the visitor to its own endpoint page.
func statusMeta(cfg *config.Config, loc, state string, s config.Service) (label, href string) {
	if state == "" {
		return "", ""
	}
	// The pill link is for the visitor's browser, so it uses the public
	// status.page URL; status.gatus may be an internal poll-only address.
	base := cfg.Site.Status.Page
	if base == "" {
		base = cfg.Site.Status.Gatus
	}
	href = strings.TrimSuffix(base, "/") + "/endpoints/" + status.Key(s.Category, s.ID)
	return cfg.Str(loc, "status."+state), href
}

// statusOf returns "", "unknown", "up" or "down". While Gatus has not
// answered yet (boot, outage) every pill is unknown; once it has, services it
// does not monitor show no pill at all.
func statusOf(cfg *config.Config, statuses map[string]bool, id string) string {
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

func BuildModel(cfg *config.Config, statuses map[string]bool) (*Model, error) {
	def := cfg.DefaultLocale()
	media := func(src string) (string, int, int) {
		d := cfg.MediaDims[src]
		return AppURL(mediaURL(src)), d[0], d[1]
	}
	pages := map[string]Page{}
	for _, loc := range cfg.Site.Locales {
		base := pageView{
			Locale:    loc,
			Prefix:    BasePath,
			Dir:       config.LocaleDir(loc),
			Base:      cfg.Site.URL + BasePath,
			Version:   Version,
			Credit:    cfg.Site.Credit == nil || *cfg.Site.Credit,
			ShowVer:   cfg.Site.ShowVer,
			SiteTitle: cfg.Site.Title.Get(loc, def),
			Logo:      AppURL(cfg.Site.Logo),
			Favicon:   AppURL(cfg.Site.Favicon),
			TouchIcon: AppURL(TouchIcon(cfg.Site.Favicon)),
			OGImage:   ogImage(cfg.Site.URL+BasePath, AppURL(cfg.Site.Logo)),
			Noindex:   cfg.Noindex(),
			AboutHash: aboutHash(cfg.Site.About),
			Accent:    cfg.Site.Theme.Accent,
			CustomCSS: cfg.CustomCSS,
			Locales:   cfg.Site.Locales,
			S: uiStrings{
				Skip:              cfg.Str(loc, "nav.skip"),
				Languages:         cfg.Str(loc, "nav.languages"),
				SearchLabel:       cfg.Str(loc, "search.label"),
				SearchPlaceholder: cfg.Str(loc, "search.placeholder"),
				SearchEmpty:       cfg.Str(loc, "search.empty"),
				SearchOne:         cfg.Str(loc, "search.one"),
				SearchMany:        cfg.Str(loc, "search.many"),
				Open:              cfg.Str(loc, "detail.open"),
				Back:              cfg.Str(loc, "detail.back"),
				More:              cfg.Str(loc, "card.more"),
				Link:              cfg.Str(loc, "status.link"),
				Toc:               cfg.Str(loc, "nav.toc"),
				LinksLabel:        cfg.Str(loc, "nav.links"),
				Dismiss:           cfg.Str(loc, "about.dismiss"),
				Theme:             cfg.Str(loc, "nav.theme"),
				Powered:           cfg.Str(loc, "foot.powered"),
				Menu:              cfg.Str(loc, "nav.menu"),
				Top:               cfg.Str(loc, "nav.top"),
			},
		}
		base.VerLabel, base.VerHref = versionInfo(Version)
		for _, l := range cfg.Site.Links {
			lv := linkView{Label: l.Label.Get(loc, def), URL: l.URL}
			linkIcon(&lv, l.Icon)
			base.Links = append(base.Links, lv)
		}
		for _, f := range cfg.Site.Footer {
			base.Footer = append(base.Footer, linkView{Label: f.Label.Get(loc, def), URL: f.URL})
		}
		for _, p := range cfg.Site.Pages {
			base.Footer = append(base.Footer, linkView{Label: p.Title.Get(loc, def), URL: BasePath + "/" + loc + "/" + p.ID + "/"})
		}

		hv := homeView{pageView: base, Tagline: cfg.Site.Tagline.Get(loc, def), About: mdBlocks(cfg.Site.About.Get(loc, def), mdCtx{media: media})}
		hv.Search = true
		hv.PageTitle = hv.SiteTitle
		hv.MetaDesc = hv.Tagline
		for _, c := range cfg.Categories {
			cv := catView{ID: c.ID, Name: cfg.CategoryName(c, loc)}
			for _, s := range c.Services {
				card := cardView{
					URL:    s.URL,
					Icon:   AppURL(config.IconURL(cfg, s.Icon)),
					Name:   s.Name.Get(loc, def),
					Desc:   s.Desc.Get(loc, def),
					Tags:   strings.Join(s.Tags, " "),
					Status: statusOf(cfg, statuses, s.ID),
				}
				card.StatusLabel, card.StatusHref = statusMeta(cfg, loc, card.Status, s)
				if len(s.Details) > 0 || len(s.Images) > 0 {
					card.MoreHref = BasePath + "/" + loc + "/" + s.ID + "/"
				}
				if s.Selfhosted != nil {
					if *s.Selfhosted {
						card.HostKind, card.HostLabel = "self", cfg.Str(loc, "host.self")
					} else {
						card.HostKind, card.HostLabel = "external", cfg.Str(loc, "host.external")
					}
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
					pageView: base,
					Name:     s.Name.Get(loc, def),
					Desc:     s.Desc.Get(loc, def),
					Icon:     AppURL(config.IconURL(cfg, s.Icon)),
					URL:      s.URL,
					Status:   statusOf(cfg, statuses, s.ID),
					Body:     mdBlocks(s.Details.Get(loc, def), mdCtx{media: media}),
				}
				for _, img := range s.Images {
					url, w, h := media(img.Src)
					dv.Images = append(dv.Images, imageView{Src: url, Caption: img.Caption.Get(loc, def), W: w, H: h})
				}
				dv.StatusLabel, dv.StatusHref = statusMeta(cfg, loc, dv.Status, s)
				dv.PageTitle = dv.Name + " · " + base.SiteTitle
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
			body := p.Body.Get(loc, def)
			sv := staticView{pageView: base, Title: p.Title.Get(loc, def), Intro: mdBlocks(body, mdCtx{pClass: "page-intro", media: media})}
			firstSec := ""
			for _, s := range p.Sections {
				secBody := s.Body.Get(loc, def)
				if firstSec == "" {
					firstSec = secBody
				}
				sv.Sections = append(sv.Sections, sectionView{Title: s.Title.Get(loc, def), Body: mdBlocks(secBody, mdCtx{media: media})})
			}
			sv.PageTitle = sv.Title + " · " + base.SiteTitle
			if ps := paragraphs(body); len(ps) > 0 {
				sv.MetaDesc = mdText(ps[0])
			} else if ps := paragraphs(firstSec); len(ps) > 0 {
				sv.MetaDesc = mdText(ps[0])
			}
			sv.SwitchPath = p.ID + "/"
			page, err := render("page.tmpl", sv)
			if err != nil {
				return nil, fmt.Errorf("render /%s/%s/: %w", loc, p.ID, err)
			}
			pages[loc+"/"+p.ID] = page
		}
	}
	return &Model{Cfg: cfg, Pages: pages, Statuses: statuses, CSP: BuildCSP(cfg), Ready: true}, nil
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

// defaultTouchIcon is cairn's own, the one shipped in assets/.
const defaultTouchIcon = "/static/touch-icon.png"

// TouchIcon picks the add-to-home-screen icon: the operator's favicon when
// it is a png (the format phones accept), cairn's own otherwise.
func TouchIcon(favicon string) string {
	if strings.HasSuffix(strings.ToLower(favicon), ".png") {
		return favicon
	}
	return defaultTouchIcon
}

// AppIcon is one entry of the web app manifest's icon list. Everything but
// the source is omitted when unknown, rather than filled with a guess.
type AppIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes,omitempty"`
	Type    string `json:"type,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

// iconType maps a file reference to the mime type the manifest wants. An empty
// result means we do not recognise it, and the entry then omits the field
// rather than asserting a format.
func iconType(ref string) string {
	i := strings.LastIndex(ref, ".")
	if i < 0 {
		return ""
	}
	switch strings.ToLower(ref[i+1:]) {
	case "svg":
		return "image/svg+xml"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "ico":
		return "image/x-icon"
	}
	return ""
}

// AppIcons lists what a phone should use once the site is on a home screen.
//
// The rule throughout is that every field is either true or absent. Sizes are
// never invented: an entry whose size we cannot establish simply carries no
// `sizes`, which the manifest spec allows, and the browser decides. The old
// behaviour was to stamp "180x180" on whatever the operator supplied, which
// was a guess published as a fact.
//
// Three cases, in order:
//
//   - The operator listed `icons` themselves. That wins outright, sizes and
//     all, because they are the only one who can know them.
//   - They set a `favicon`. An svg is scalable, so it is declared "any" and
//     serves every size the way cairn's own does; a raster in the mounted
//     assets dir is measured at load and declared truthfully; a raster behind
//     a URL is offered with no size, since measuring it means an outbound
//     request and cairn makes none.
//   - Neither. cairn's own set, which ships the 192 and the 512 that Chromium
//     insists on before it will offer to install a site, both declared
//     "any maskable": the usual reason for a separate padded maskable file is
//     that a full-bleed icon loses its edges to Android's crop, and this mark
//     is a narrow stack that clears that circle with room to spare.
func AppIcons(cfg *config.Config) []AppIcon {
	if own := cfg.Site.Icons; len(own) > 0 {
		out := make([]AppIcon, 0, len(own))
		for _, ic := range own {
			out = append(out, AppIcon{
				Src: AppURL(ic.Src), Sizes: ic.Sizes, Type: iconType(ic.Src), Purpose: ic.Purpose,
			})
		}
		return out
	}

	if fav := cfg.Site.Favicon; fav != "" {
		icon := AppIcon{Src: AppURL(fav), Type: iconType(fav)}
		switch {
		case icon.Type == "image/svg+xml":
			icon.Sizes = "any"
		case cfg.FaviconDims != [2]int{}:
			icon.Sizes = fmt.Sprintf("%dx%d", cfg.FaviconDims[0], cfg.FaviconDims[1])
		}
		return []AppIcon{icon}
	}

	return []AppIcon{
		{Src: AppURL(defaultTouchIcon), Sizes: "180x180", Type: "image/png"},
		{Src: AppURL("/static/icon-192.png"), Sizes: "192x192", Type: "image/png", Purpose: "any maskable"},
		{Src: AppURL("/static/icon-512.png"), Sizes: "512x512", Type: "image/png", Purpose: "any maskable"},
	}
}

// ogImage derives a social preview image from the logo: only when the site
// has a public URL to make it absolute, and only raster formats, which is
// what the preview crawlers accept.
func ogImage(base, logo string) string {
	if base == "" || logo == "" {
		return ""
	}
	l := strings.ToLower(logo)
	if !strings.HasSuffix(l, ".png") && !strings.HasSuffix(l, ".jpg") && !strings.HasSuffix(l, ".jpeg") && !strings.HasSuffix(l, ".webp") && !strings.HasSuffix(l, ".gif") {
		return ""
	}
	if config.IsLocalPath(logo) {
		return base + logo
	}
	return logo
}

// mediaURL resolves a bare image name against the /media/ route, which
// serves the config dir's media/ folder; URLs and absolute paths pass
// through, same convention as icons.
func mediaURL(src string) string {
	if config.IsURLOrAbs(src) {
		return src
	}
	return "/media/" + src
}

// Version is what the footer shows when show_version is on. Package main
// carries the -ldflags stamp and assigns it here at startup.
var Version = "dev"

// The repository the credit and the version stamp point at.
const repoURL = "https://github.com/MorganKryze/cairn"

var (
	semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// versionInfo turns the build stamp into what the footer shows and where it
// points, which depends on where the binary came from: a tagged release links
// to its release notes, a build off main links to the commit it was cut from,
// and anything else (a local `go build`, which stamps nothing) is named but
// not linked, since there is no public page for it.
func versionInfo(v string) (label, href string) {
	switch {
	case semverRe.MatchString(v):
		return v, repoURL + "/releases/tag/v" + v
	case commitRe.MatchString(v):
		return "@" + v[:7], repoURL + "/commit/" + v
	default:
		return v, ""
	}
}
