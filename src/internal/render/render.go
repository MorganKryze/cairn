package render

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"sort"
	"strconv"
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
// "fr/pdf" for a detail. Statuses is service-id -> what Gatus said about it.
type Model struct {
	Cfg      *config.Config
	Pages    map[string]Page
	Statuses map[string]status.State
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
// data-js is stamped here rather than from a deferred script because CSS keys
// off it before first paint: with JavaScript off, the mobile category trail
// must stay as chips instead of folding into a select only nav.js can drive.
const prePaintScript = `document.documentElement.setAttribute('data-js','');var a=document.cookie.match(/(?:^|; )about=([^;]*)/);if(a&&a[1]===document.documentElement.getAttribute('data-about'))document.documentElement.setAttribute('data-noabout','');try{var t=localStorage.getItem('theme');if(t)document.documentElement.setAttribute('data-theme',t)}catch(e){}`

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
	csp := "default-src 'none'; img-src 'self' https: data:; " +
		"style-src 'self' " + cspHash(accentStyle(cfg.Site.Theme.Accent)) + "; " +
		"script-src 'self' " + cspHash(prePaintScript) + "; " +
		"font-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	// The one request a cairn page ever makes from the browser: status.js
	// refetching the page it is on. Sites with no Gatus do not ship the script
	// and do not get the permission.
	if cfg.Site.StatusAddress() != "" {
		csp += "; connect-src 'self'"
	}
	return csp
}

// locText is one translated string together with the language it turned out
// to be written in.
//
// A field with no translation for the page's locale still renders, in
// whichever language the config does have, and the page used to claim through
// <html lang> that the sentence was in its own: a screen reader then reads it
// in the wrong voice, and an Arabic fallback inside a left-to-right page is
// laid out the wrong way round, punctuation and all. Attrs is the ` lang="…"
// dir="…"` that says otherwise, and a template opts into it element by
// element.
//
// Printing is unchanged. String makes {{.Field}} render the wording and
// nothing else, so every template that only prints keeps printing what it did.
// Testing is not: this is a struct, and {{if .Field}} on a struct is always
// true, so a template that shows an element only when the text exists has to
// ask for {{if .Field.Text}}. TestEmptyFieldsStillRenderNothing is what
// catches the slip, since the symptom is an empty element nobody looks at.
//
// Both Lang and Dir are empty in the overwhelmingly common case, a site whose
// fields are translated for the locales it lists. A mark on every field would
// be worse than none: it would say "foreign language" so often that the times
// it is true would stop meaning anything.
type locText struct {
	Text string
	Lang string // the text's own locale, empty when it is the page's language
	Dir  string // "ltr"/"rtl", empty unless the text reads the other way round
}

func (t locText) String() string { return t.Text }

func (t locText) Attrs() template.HTMLAttr { return langAttrs(t.Lang, t.Dir) }

// prose is the same thing for a markdown field: the blocks it rendered to,
// and the language they are written in when it is not the page's.
type prose struct {
	Blocks    []template.HTML
	Lang, Dir string
}

func (p prose) Attrs() template.HTMLAttr { return langAttrs(p.Lang, p.Dir) }

// langAttrs builds the attribute pair a marked run of text carries.
//
// template.HTMLAttr is an escape hatch: html/template copies it into the tag
// verbatim, so a forgeable value here would be an injection. Nothing reaching
// it is forgeable. lang is a locale tag, and IsLocale is checked here rather
// than assumed from load, so the guarantee holds even if some later field
// arrives by another road: localeRe is ^[a-zA-Z][a-zA-Z0-9-]*$, which admits
// letters, digits and dashes and therefore no quote, space, angle bracket or
// equals sign. dir is compared against the only two values it may ever hold.
// Anything failing either check is dropped rather than escaped: a page with no
// mark is merely as wrong as it was before this existed, while a page with a
// forged attribute is a different kind of wrong.
//
// dir is left out when it matches the page's own direction. English text on a
// French page reads left to right either way, and dir="ltr" on every such
// field would be noise restating what the browser already inherited.
func langAttrs(lang, dir string) template.HTMLAttr {
	if !config.IsLocale(lang) {
		return ""
	}
	attrs := ` lang="` + lang + `"`
	if dir == "ltr" || dir == "rtl" {
		attrs += ` dir="` + dir + `"`
	}
	return template.HTMLAttr(attrs)
}

// tr resolves one translated field for the page being built: loc is the
// page's locale, def the site default that the lookup falls back through.
func tr(ls config.LString, loc, def string) locText {
	text, from := ls.GetLocale(loc, def)
	t := locText{Text: text}
	if text == "" || config.SameLanguage(from, loc) {
		return t
	}
	t.Lang = from
	if d := config.LocaleDir(from); d != config.LocaleDir(loc) {
		t.Dir = d
	}
	return t
}

// proseOf renders a translated markdown field, keeping the language mark with
// the blocks it produced.
func proseOf(t locText, ctx mdCtx) prose {
	return prose{Blocks: mdBlocks(t.Text, ctx), Lang: t.Lang, Dir: t.Dir}
}

// catName is CategoryName with the marking, and only where there is something
// to mark. The name an operator wrote is a translation and falls back like any
// other; the two cairn derives instead (the localized "Other" bucket, and a
// category id title-cased into a heading) are in the page's language or in no
// language at all, so neither carries a mark.
func catName(cfg *config.Config, c config.Category, loc string) locText {
	if len(c.Name) > 0 {
		if t := tr(c.Name, loc, cfg.DefaultLocale()); t.Text != "" {
			return t
		}
	}
	return locText{Text: cfg.CategoryName(c, loc)}
}

type uiStrings struct {
	Skip, Languages, SearchLabel, SearchPlaceholder, SearchEmpty, SearchOne, SearchMany, Open, Back, More, Link, Toc, LinksLabel, Dismiss, Theme, Powered, Menu, Top string
}

type pageView struct {
	Locale, Logo, Favicon, TouchIcon, OGImage, Accent, SwitchPath, Base, Version, AboutHash string
	// PageTitle and MetaDesc are the <title> and the meta/og description. Both
	// are text inside markup rather than markup, so neither can carry a lang
	// attribute, and neither can be split into a marked part and an unmarked
	// one: a title is one string. A fallback shows there unannounced, and the
	// only honest thing to do about it is not pretend otherwise. Same for the
	// og:title and the aria-label statusMeta builds.
	PageTitle, MetaDesc                         string
	SiteTitle                                   locText
	Prefix                                      string // "" or "/cairn", see BasePath
	Dir                                         string // "ltr" or "rtl", from the locale
	CustomCSS, Search, Credit, Noindex, ShowVer bool
	// StatusPoll is how often, in seconds, status.js refetches the page to
	// swap the pills. Zero on a site with no Gatus, where the script does not
	// ship at all.
	StatusPoll        int
	VerLabel, VerHref string
	Locales           []string
	Links             []linkView
	Footer            []linkView
	S                 uiStrings
}

type cardView struct {
	URL, Icon, Tags, MoreHref, Status, StatusLabel, StatusHref string
	Name, Desc                                                 locText
	StatusA11y                                                 string // set only when the pill is a link
	StatusID                                                   string // names the pill's slot, empty without a Gatus
	HostKind, HostLabel                                        string // "self"/"external"/"" and its localized label
}

type catView struct {
	ID    string
	Name  locText
	Cards []cardView
}

type linkView struct {
	Label   locText
	URL     string
	IconSVG template.HTML // one of config.Glyphs, trusted markup
	IconIMG string        // user-supplied URL or /assets path
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
	Tagline locText
	About   prose
	Cats    []catView
}

type detailView struct {
	pageView
	StatusID                                   string
	Icon, URL, Status, StatusLabel, StatusHref string
	Name, Desc                                 locText
	StatusA11y                                 string
	Body                                       prose
	Images                                     []imageView
}

type imageView struct {
	Src     string
	Caption locText
	W, H    int
}

type staticView struct {
	pageView
	Title    locText
	Intro    prose
	Sections []sectionView
}

type sectionView struct {
	Title locText
	Body  prose
}

// statusBase is the page a pill points into, without a trailing slash.
//
// The link is for the visitor's browser, so status.page wins: the address
// cairn polls may be internal and poll-only, and only the operator knows.
// Failing that, Kuma's published status page is derivable, since /status/{slug}
// is the URL Kuma itself serves it at, and landing on a status page beats
// landing on an API root that redirects to a login.
func statusBase(cfg *config.Config) string {
	if p := cfg.Site.Status.Page; p != "" {
		return strings.TrimSuffix(p, "/")
	}
	addr := strings.TrimSuffix(cfg.Site.StatusAddress(), "/")
	if cfg.Site.StatusProvider() == "kuma" && cfg.Site.Status.Slug != "" {
		return addr + "/status/" + url.PathEscape(cfg.Site.Status.Slug)
	}
	return addr
}

// statusMeta fills a pill: its label, where it links, and the label a screen
// reader reads instead. An empty href makes it display-only, which is what
// status.linked: false asks for and the only shape the templates need to
// tell the two apart.
func statusMeta(cfg *config.Config, loc, state string, s config.Service, key string) (label, href, a11y string) {
	if state == "" {
		return "", "", ""
	}
	label = cfg.Str(loc, "status."+state)
	if !cfg.StatusLinked() {
		return label, "", ""
	}
	base := statusBase(cfg)
	// Only Gatus names a page per endpoint. Every other monitor publishes one
	// page for the whole set, and pointing at a per-service path it does not
	// serve is the bug v1.13.2 fixed for Gatus, made again from the other end.
	if cfg.Site.StatusProvider() == "gatus" {
		// Gatus names its own endpoint pages. Deriving that name from cairn's
		// categories only ever matched an operator who ran -emit-gatus and kept its
		// grouping; everyone else got a link to a page their Gatus does not serve.
		// The derived key stays as the fallback for the pills that render before
		// Gatus has answered, and for a Gatus too old to report one.
		if key == "" {
			key = status.Key(s.Category, s.ID)
		}
		// The key comes off the network, from whatever answers on the poll address.
		// PathEscape leaves every key Gatus can produce alone and keeps a hostile
		// one inside the path segment it was written into.
		base += "/endpoints/" + url.PathEscape(key)
	}
	href = base
	// A link has to name its target on its own: read out of the card, "Online"
	// alone says nothing about which service, nor that it leads anywhere.
	//
	// The service name here may be a fallback in another language, and this one
	// cannot be marked: an aria-label is text inside an attribute, so there is
	// no element to hang lang and dir on, and the name is glued to two strings
	// in the page's own language besides. The visible pill next to it carries
	// the marking the label cannot.
	return label, href, s.Name.Get(loc, cfg.DefaultLocale()) + ", " + label + ", " + cfg.Str(loc, "status.link")
}

// statusOf returns "", "unknown", "up", "degraded", "maintenance" or "down".
// While the monitor has not answered yet (boot, outage) every pill is unknown;
// once it has, services it does not monitor show no pill at all.
//
// Only two of them are reachable from Gatus, whose results carry a success
// bool and nothing else. The other two arrive from monitors that say more.
func statusOf(cfg *config.Config, statuses map[string]status.State, id string) string {
	if cfg.Site.StatusAddress() == "" {
		return ""
	}
	if len(statuses) == 0 {
		return "unknown"
	}
	st, ok := statuses[id]
	switch {
	case !ok:
		return ""
	case st.Level == status.LevelMaintenance:
		return "maintenance"
	case st.Level == status.LevelDegraded:
		return "degraded"
	case st.Level == status.LevelUp:
		return "up"
	default:
		// Every other level, including one no version of cairn has heard of,
		// reads as down. That is the safe direction: a source that grows a word
		// must not be able to paint a broken service green.
		return "down"
	}
}

// statusSlot names the pill's slot, which is in the markup on every service of
// a site that has a Gatus, pill or no pill. status.js swaps what is inside it,
// and one of the states it has to be able to reach is "nothing": a service the
// status page stops monitoring loses its pill rather than keeping a stale one.
func statusSlot(cfg *config.Config, id string) string {
	if cfg.Site.StatusAddress() == "" {
		return ""
	}
	return id
}

// statusPoll is how often status.js refetches the page, in seconds. It is the
// interval cairn polls Gatus on: asking cairn more often than cairn can learn
// anything new would only cost requests.
func statusPoll(cfg *config.Config) int {
	if cfg.Site.StatusAddress() == "" {
		return 0
	}
	return int(cfg.StatusInterval().Seconds())
}

func BuildModel(cfg *config.Config, statuses map[string]status.State) (*Model, error) {
	def := cfg.DefaultLocale()
	media := func(src string) (string, int, int) {
		// MediaDims is keyed by the name under media/. The /media/… spelling
		// reaches the same file and used to miss the lookup, so the width and
		// height that exist to stop the page jumping were dropped for it.
		key := src
		if rel, ok := strings.CutPrefix(src, "/media/"); ok {
			key = rel
		}
		d := cfg.MediaDims[key]
		return AppURL(mediaURL(src)), d[0], d[1]
	}
	pages := map[string]Page{}
	for _, loc := range cfg.Site.Locales {
		base := pageView{
			Locale:     loc,
			Prefix:     BasePath,
			Dir:        config.LocaleDir(loc),
			Base:       absBase(cfg),
			Version:    Version,
			Credit:     cfg.Site.Credit == nil || *cfg.Site.Credit,
			ShowVer:    cfg.Site.ShowVer,
			StatusPoll: statusPoll(cfg),
			SiteTitle:  tr(cfg.Site.Title, loc, def),
			Logo:       AppURL(cfg.Site.Logo),
			Favicon:    AppURL(cfg.Site.Favicon),
			TouchIcon:  AppURL(TouchIcon(cfg)),
			// AppURL has already added BasePath to a local logo, so the base
			// passed here must not carry it again: it used to, and every
			// og:image under -base-path pointed at /cairn/cairn/… and 404ed.
			OGImage:   ogImage(cfg.Site.URL, AppURL(cfg.Site.Logo)),
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
			lv := linkView{Label: tr(l.Label, loc, def), URL: l.URL}
			linkIcon(&lv, l.Icon)
			base.Links = append(base.Links, lv)
		}
		for _, f := range cfg.Site.Footer {
			base.Footer = append(base.Footer, linkView{Label: tr(f.Label, loc, def), URL: f.URL})
		}
		for _, p := range cfg.Site.Pages {
			base.Footer = append(base.Footer, linkView{Label: tr(p.Title, loc, def), URL: BasePath + "/" + loc + "/" + p.ID + "/"})
		}

		hv := homeView{
			pageView: base,
			Tagline:  tr(cfg.Site.Tagline, loc, def),
			About:    proseOf(tr(cfg.Site.About, loc, def), mdCtx{media: media}),
		}
		hv.Search = true
		hv.PageTitle = hv.SiteTitle.Text
		hv.MetaDesc = hv.Tagline.Text
		for _, c := range cfg.Categories {
			cv := catView{ID: c.ID, Name: catName(cfg, c, loc)}
			for _, s := range c.Services {
				card := cardView{
					URL:      s.URL,
					Icon:     AppURL(config.IconURL(cfg, s.Icon)),
					Name:     tr(s.Name, loc, def),
					Desc:     tr(s.Desc, loc, def),
					Tags:     strings.Join(s.Tags, " "),
					Status:   statusOf(cfg, statuses, s.ID),
					StatusID: statusSlot(cfg, s.ID),
				}
				card.StatusLabel, card.StatusHref, card.StatusA11y = statusMeta(cfg, loc, card.Status, s, statuses[s.ID].Key)
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
					Name:     tr(s.Name, loc, def),
					Desc:     tr(s.Desc, loc, def),
					Icon:     AppURL(config.IconURL(cfg, s.Icon)),
					URL:      s.URL,
					Status:   statusOf(cfg, statuses, s.ID),
					StatusID: statusSlot(cfg, s.ID),
					Body:     proseOf(tr(s.Details, loc, def), mdCtx{media: media}),
				}
				for _, img := range s.Images {
					url, w, h := media(img.Src)
					dv.Images = append(dv.Images, imageView{Src: url, Caption: tr(img.Caption, loc, def), W: w, H: h})
				}
				dv.StatusLabel, dv.StatusHref, dv.StatusA11y = statusMeta(cfg, loc, dv.Status, s, statuses[s.ID].Key)
				dv.PageTitle = dv.Name.Text + " · " + base.SiteTitle.Text
				dv.MetaDesc = dv.Desc.Text
				dv.SwitchPath = s.ID + "/"
				page, err := render("detail.tmpl", dv)
				if err != nil {
					return nil, fmt.Errorf("render /%s/%s/: %w", loc, s.ID, err)
				}
				pages[loc+"/"+s.ID] = page
			}
		}

		for _, p := range cfg.Site.Pages {
			intro := tr(p.Body, loc, def)
			body := intro.Text
			sv := staticView{pageView: base, Title: tr(p.Title, loc, def), Intro: proseOf(intro, mdCtx{pClass: "page-intro", media: media})}
			firstSec := ""
			for _, s := range p.Sections {
				sec := tr(s.Body, loc, def)
				if firstSec == "" {
					firstSec = sec.Text
				}
				sv.Sections = append(sv.Sections, sectionView{Title: tr(s.Title, loc, def), Body: proseOf(sec, mdCtx{media: media})})
			}
			sv.PageTitle = sv.Title.Text + " · " + base.SiteTitle.Text
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

// widestSize reads the largest width out of a manifest `sizes` value, which
// may list several ("48x48 96x96") or be "any". Anything unreadable is 0, so
// it loses to any real number without needing a separate case.
func widestSize(sizes string) int {
	best := 0
	for _, token := range strings.Fields(sizes) {
		w, _, ok := strings.Cut(token, "x")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(w); err == nil && n > best {
			best = n
		}
	}
	return best
}

// TouchIcon picks the add-to-home-screen icon.
//
// iOS never reads the manifest for this: it reads the apple-touch-icon link
// and nothing else. So an operator who listed their own icons has to be
// honoured here too, or their list would take effect on Android while an
// iPhone home screen showed cairn's mark, which is the one thing supplying
// your own icons is meant to prevent. The largest is picked, since iOS
// downsamples and only a too-small icon shows.
func TouchIcon(cfg *config.Config) string {
	if own := cfg.Site.Icons; len(own) > 0 {
		best, bestPx := own[0].Src, widestSize(own[0].Sizes)
		for _, ic := range own[1:] {
			if px := widestSize(ic.Sizes); px > bestPx {
				best, bestPx = ic.Src, px
			}
		}
		return best
	}
	if strings.HasSuffix(strings.ToLower(cfg.Site.Favicon), ".png") {
		return cfg.Site.Favicon
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
// IsRaster reports a format a link previewer can actually display. svg is
// deliberately out: og:image with a vector is ignored by most platforms, which
// is why a vector logo quietly means no preview picture at all. -check says so
// rather than leaving it to be discovered on a shared link.
func IsRaster(path string) bool {
	l := strings.ToLower(path)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		if strings.HasSuffix(l, ext) {
			return true
		}
	}
	return false
}

// absBase is the public origin every self-referencing link is built from, and
// it is empty unless the operator gave one. Adding BasePath to an empty url
// left a bare "/cairn", which is truthy in the template and made cairn emit
// relative canonical, og:url and hreflang links: worse than emitting none,
// which is what the absence of a url is supposed to mean.
func absBase(cfg *config.Config) string {
	if cfg.Site.URL == "" {
		return ""
	}
	return cfg.Site.URL + BasePath
}

func ogImage(base, logo string) string {
	if base == "" || logo == "" {
		return ""
	}
	if !IsRaster(logo) {
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
