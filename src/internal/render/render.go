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
	// asset resolves one of cairn's own files to the stamped name it is served
	// under, base path included, so a template never writes /static/ by hand and
	// never has to remember .Prefix for one. See asseturl.go.
	"asset": AssetURL,
}).ParseFS(Embedded, "templates/*.tmpl"))

type Page struct {
	HTML []byte
	ETag string
}

// Model is the immutable unit swapped atomically on config reload or status
// change. Pages is keyed by URL path without surrounding slashes ("fr" for a
// home, "fr/pdf" for a detail), Statuses by service id.
type Model struct {
	Cfg      *config.Config
	Pages    map[string]Page
	Statuses map[string]status.State
	CSP      string
	// Ready is false only while the getting-started page stands in for a config
	// that never loaded. /readyz reports it, /healthz does not, so a liveness
	// probe cannot restart-loop a container that is serving fine.
	Ready bool
}

// prePaintScript must stay byte-identical to the inline script in layout.tmpl:
// its hash is what the CSP allows, and a test guards the match. The about cookie
// holds a hash of the note's content (the data-about attribute), so an edited
// note reappears even for visitors who dismissed the previous one. data-js is
// stamped here rather than from a deferred script because CSS keys off it before
// first paint: with JavaScript off, the mobile category trail must stay as chips
// instead of folding into a select only nav.js can drive.
const prePaintScript = `document.documentElement.setAttribute('data-js','');var a=document.cookie.match(/(?:^|; )about=([^;]*)/);if(a&&a[1]===document.documentElement.getAttribute('data-about'))document.documentElement.setAttribute('data-noabout','');try{var t=localStorage.getItem('theme');if(t)document.documentElement.setAttribute('data-theme',t)}catch(e){}`

// aboutHash fingerprints the welcome note across all locales. Eight hex chars
// are enough for a cookie value that only has to change when the note does.
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

// themeStyle is the inline <style> every page carries: the accent on :root (the
// stylesheet's own value is a placeholder), the body font when
// theme.font.family names one, and the self-hosted @font-face when
// theme.font.file does. Its hash is what the CSP allows, so this string and the
// <style> layout.tmpl renders have to stay identical; csp_test.go guards the
// match.
func themeStyle(cfg *config.Config) string {
	var b strings.Builder
	if f := cfg.Site.Theme.Font.File; f != "" {
		if rel, ok := config.FontRef(f); ok {
			// No font-weight descriptor. A file dropped in fonts/ is usually one
			// static weight, and a face claiming to cover 100 to 900 is matched
			// at every weight, so the browser stops synthesizing bold: the 470
			// through 650 the page sets on names, headings and the current entry
			// of a table of contents would all come out flat. A variable font
			// measures the same with the descriptor and without, on Chromium and
			// WebKit alike.
			fmt.Fprintf(&b, "@font-face{font-family:%s;src:url(%q) format(%q);font-style:normal;font-display:swap}",
				config.FontFaceName(cfg.Site.Theme.Font.Family),
				AppURL("/fonts/"+rel),
				config.FontFormat(rel))
		}
	}
	b.WriteString(":root{--accent:")
	b.WriteString(cfg.Site.Theme.Accent)
	if fam := cfg.Site.Theme.Font.Family; fam != "" {
		b.WriteString(";--font-body:")
		b.WriteString(fam)
	}
	b.WriteString("}")
	// Last: a site that themes no artwork has to produce the same string, and so
	// the same CSP hash, as one built before this existed.
	var logoDark string
	if l := cfg.Site.Logo; l.Themed() {
		logoDark = AppURL(l.Dark)
	}
	b.WriteString(themedFor(cfg).rules(logoDark))
	return b.String()
}

func cspHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// A site with no monitor ships no status.js, no pills and no slots for one.
func hasMonitor(cfg *config.Config) bool { return cfg.Site.StatusAddress() != "" }

// BuildCSP allows exactly what the pages use: self assets, the inline style
// (accent, and the @font-face and body font when theme.font is set), the known
// inline script by hash, and images from anywhere https (icon slugs).
func BuildCSP(cfg *config.Config) string {
	csp := "default-src 'none'; img-src 'self' https: data:; " +
		"style-src 'self' " + cspHash(themeStyle(cfg)) + "; " +
		"script-src 'self' " + cspHash(prePaintScript) + "; " +
		"font-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	// The one request a cairn page ever makes from the browser: status.js
	// refetching the page it is on.
	if hasMonitor(cfg) {
		csp += "; connect-src 'self'"
	}
	return csp
}

// locText is one translated string together with the language it turned out to
// be written in.
//
// A field with no translation for the page's locale still renders, in whichever
// language the config does have, while <html lang> claims it is in the page's
// own: a screen reader then reads it in the wrong voice, and an Arabic fallback
// inside a left-to-right page is laid out the wrong way round, punctuation and
// all. Attrs is the ` lang="…" dir="…"` that says otherwise, and a template opts
// into it element by element.
//
// String makes {{.Field}} render the wording and nothing else, so a template
// that only prints keeps printing what it did. Testing differs: this is a
// struct, and {{if .Field}} on a struct is always true, so a template that shows
// an element only when the text exists has to ask for {{if .Field.Text}}.
// TestEmptyFieldsStillRenderNothing catches the slip, since the symptom is an
// empty element nobody looks at.
type locText struct {
	Text string
	Lang string // the text's own locale, empty when it is the page's language
	Dir  string // "ltr"/"rtl", empty unless the text reads the other way round
}

func (t locText) String() string { return t.Text }

func (t locText) Attrs() template.HTMLAttr { return langAttrs(t.Lang, t.Dir) }

// prose is locText for a markdown field: the blocks it rendered to, and the
// language they are written in when it is not the page's.
type prose struct {
	Blocks    []template.HTML
	Lang, Dir string
}

func (p prose) Attrs() template.HTMLAttr { return langAttrs(p.Lang, p.Dir) }

// langAttrs builds the attribute pair a marked run of text carries.
//
// template.HTMLAttr is an escape hatch: html/template copies it into the tag
// verbatim, so a forgeable value here would be an injection. IsLocale is checked
// here rather than assumed from load, so the guarantee holds even for a field
// that arrives by some later road: localeRe is ^[a-zA-Z][a-zA-Z0-9-]*$, which
// admits letters, digits and dashes and therefore no quote, space, angle bracket
// or equals sign. dir is compared against the only two values it may ever hold.
// Anything failing either check is dropped rather than escaped.
//
// dir is left out when it matches the page's own direction, which the browser
// has already inherited.
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

func proseOf(t locText, ctx mdCtx) prose {
	return prose{Blocks: mdBlocks(t.Text, ctx), Lang: t.Lang, Dir: t.Dir}
}

// catName marks only a name an operator wrote, which falls back like any other
// translation. The two cairn derives instead, the localized "Other" bucket and a
// category id title-cased into a heading, are in the page's language or in no
// language at all.
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
	// The six the leave dialog needs, empty on a site that does not ask for it.
	// LeaveCopied is the label the copy button swaps to, so it goes into a data
	// attribute rather than the button: the script needs both.
	LeaveTitle, LeaveBody, LeaveCopy, LeaveCopied, LeaveGo, LeaveStay string
}

type pageView struct {
	Locale, Logo, Favicon, TouchIcon, OGImage, SwitchPath, Base, Version, AboutHash string
	// FaviconDark is the tab icon for a dark system, empty without one. It is the
	// one themed surface the stylesheet cannot reach, so it goes out as a second
	// <link> and follows the system rather than the theme button.
	FaviconDark string
	// PageTitle and MetaDesc are the <title> and the meta/og description: text
	// inside markup rather than markup, with no element to hang lang and dir on
	// and no way to split into a marked part and an unmarked one. A fallback
	// shows there unannounced. Same for og:title and the aria-label statusMeta
	// builds.
	PageTitle, MetaDesc string
	// Style is the inline <style> of every page, pre-built so its CSP hash covers
	// the same string the template renders. template.CSS tells html/template this
	// block needs no CSS-escaping: the accent is validated to a hex color and the
	// font family to the characters a CSS font stack is made of.
	Style                                       template.CSS
	SiteTitle                                   locText
	Prefix                                      string // "" or "/cairn", see BasePath
	Dir                                         string // "ltr" or "rtl", from the locale
	CustomCSS, Search, Credit, Noindex, ShowVer bool
	// StatusPoll is how often, in seconds, status.js refetches the page to swap
	// the pills. Zero on a site with no monitor, where the script never ships.
	StatusPoll        int
	VerLabel, VerHref string
	Locales           []string
	Links             []linkView
	Footer            []linkView
	S                 uiStrings
	// Leave ships the dialog and its script, per page rather than per site: a
	// page with nothing to guard gets neither. LeaveBlank makes the dialog's own
	// continue link agree with new_tab.
	Leave, LeaveBlank bool
}

type cardView struct {
	URL, Icon, Tags, MoreHref, Status, StatusLabel, StatusHref string
	// IconClass carries the dark variant of a themed icon, empty otherwise. The
	// class names the pair rather than the service, so cards sharing an icon
	// share the one rule in the page's stylesheet.
	IconClass string
	// MoreA11y names the detail link for a screen reader. The link is a glyph,
	// which says nothing on its own, and a page of identical "Learn more" links
	// is a link list nobody can navigate: it names the service too.
	MoreA11y            string
	Name, Desc          locText
	StatusA11y          string // set only when the pill is a link
	StatusID            string // names the pill's slot, empty without a monitor
	HostKind, HostLabel string // "self"/"external"/"" and its localized label
	// State is the declared word, `soon` and friends, used as a class, and
	// StateLabel is that word in the page's language. Neither has anything to do
	// with a status level.
	State, StateLabel string
	// Off is the two disabling states, hoisted onto the view so the template asks
	// one question instead of comparing strings in three places.
	Off bool
	// HostHref is where the flag leads, empty when it leads nowhere and the flag
	// stays plain text. HostBlank is set for a target that is not this site.
	HostHref  string
	HostBlank bool
	// Blank and Leave describe the link on the service's own name. Both come from
	// service_links and both only apply to a url that leaves the site, so a
	// service whose url is a page cairn serves gets neither.
	Blank, Leave bool
}

// hostKindOf reads the card's flag off the service rather than off the built
// card, so the detail page can reach the same verdict without one.
func hostKindOf(s config.Service) string {
	if s.Selfhosted == nil {
		return ""
	}
	if *s.Selfhosted {
		return "self"
	}
	return "external"
}

// serviceLink decides how one service url behaves, gated throughout on leaving
// the site: a url that is a path cairn serves is not a departure, and a "you are
// leaving" dialog in front of the operator's own page is not a warning. hostKind
// is the card's own flag, which is what lets a site warn about other people's
// services without warning about its own.
func serviceLink(cfg *config.Config, url, hostKind string) (blank, leave bool) {
	if config.IsLocalPath(url) {
		return false, false
	}
	sl := cfg.Site.ServiceLinks
	return sl.NewTab, sl.Confirm.Wants(hostKind)
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
	IconClass                                  string // see cardView.IconClass
	Name, Desc                                 locText
	StatusA11y                                 string
	Body                                       prose
	Images                                     []imageView
	Blank, Leave                               bool   // see cardView
	State, StateLabel                          string // see cardView
	Off                                        bool   // see cardView
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

// hostTarget resolves where a hosting flag leads, and whether that is off this
// site. The key is a page id rather than a path because the path is built in the
// language being read: writing /en/hosting/ would pin one language for every
// visitor. Anything else is used as written.
func hostTarget(cfg *config.Config, loc, ref string) (href string, blank bool) {
	if ref == "" {
		return "", false
	}
	for _, p := range cfg.Site.Pages {
		if p.ID == ref {
			return BasePath + "/" + loc + "/" + p.ID + "/", false
		}
	}
	return ref, !config.IsLocalPath(ref)
}

// statusBase is the page a pill points into, without a trailing slash.
//
// The link is for the visitor's browser, so status.page wins: the address cairn
// polls may be internal and poll-only, and only the operator knows. Failing
// that, Kuma serves its published page at /status/{slug}, which at least beats
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
// status.linked: false asks for and the only shape the templates need to tell
// the two apart.
func statusMeta(cfg *config.Config, loc, level string, s config.Service, key string) (label, href, a11y string) {
	if level == "" {
		return "", "", ""
	}
	label = cfg.Str(loc, "status."+level)
	if !cfg.StatusLinked() {
		return label, "", ""
	}
	base := statusBase(cfg)
	// Only Gatus names a page per endpoint. Every other monitor publishes one
	// page for the whole set, and pointing at a per-service path it does not
	// serve is the bug v1.13.2 fixed for Gatus, made again from the other end.
	if cfg.Site.StatusProvider() == "gatus" {
		// Gatus reports the key of its own endpoint page. Deriving one from
		// cairn's categories only matched an operator who ran -emit-gatus and kept
		// its grouping, so it stays as the fallback for the pills that render
		// before Gatus has answered, and for a Gatus too old to report one.
		if key == "" {
			key = status.Key(s.Category, s.ID)
		}
		// The key comes off the network, from whatever answers on the poll
		// address. PathEscape leaves every key Gatus can produce alone and keeps a
		// hostile one inside the path segment it was written into.
		base += "/endpoints/" + url.PathEscape(key)
	}
	href = base
	// A link has to name its target on its own: read out of the card, "Online"
	// alone says nothing about which service, nor that it leads anywhere. The
	// service name may be a fallback in another language and cannot be marked
	// here, an aria-label being text inside an attribute; the visible pill next
	// to it carries the marking.
	return label, href, s.Name.Get(loc, cfg.DefaultLocale()) + ", " + label + ", " + cfg.Str(loc, "status.link")
}

// statusOf returns "", "unknown", "up", "degraded", "maintenance" or "down".
// While the monitor has not answered yet (boot, outage) every pill is unknown;
// once it has, services it does not monitor show no pill at all. Gatus reaches
// only up and down, its results carrying a success bool and nothing else.
func statusOf(cfg *config.Config, statuses map[string]status.State, id string) string {
	if !hasMonitor(cfg) {
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
		// Every other level, one no version of cairn has heard of included, reads
		// as down: a source that grows a word must not be able to paint a broken
		// service green.
		return "down"
	}
}

// statusSlot names the pill's slot, which is in the markup on every service of a
// monitored site, pill or no pill. status.js swaps what is inside it, and one of
// the states it has to reach is nothing at all: a service the status page stops
// monitoring loses its pill rather than keeping a stale one.
func statusSlot(cfg *config.Config, id string) string {
	if !hasMonitor(cfg) {
		return ""
	}
	return id
}

// statusPoll is the interval cairn polls the monitor on: asking cairn more often
// than cairn can learn anything new would only cost requests.
func statusPoll(cfg *config.Config) int {
	if !hasMonitor(cfg) {
		return 0
	}
	return int(cfg.StatusInterval().Seconds())
}

func BuildModel(cfg *config.Config, statuses map[string]status.State) (*Model, error) {
	def := cfg.DefaultLocale()
	media := func(src string) (string, int, int) {
		// MediaDims is keyed by the bare name under media/, so the /media/…
		// spelling has to be cut back to it or the width and height that stop the
		// page jumping are dropped.
		key := src
		if rel, ok := strings.CutPrefix(src, "/media/"); ok {
			key = rel
		}
		d := cfg.MediaDims[key]
		return AppURL(mediaURL(src)), d[0], d[1]
	}
	// Built once rather than per locale: the set is a property of the config, not
	// of the language. themeStyle calls themedFor too and its output lands inside
	// the CSP hash, so the function has to stay deterministic.
	themed := themedFor(cfg)
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
			Logo:       AppURL(cfg.Site.Logo.Light),
			Favicon:    AppURL(cfg.Site.Favicon.Light),
			FaviconDark: func() string {
				if f := cfg.Site.Favicon; f.Themed() {
					return AppURL(f.Dark)
				}
				return ""
			}(),
			TouchIcon: AppURL(TouchIcon(cfg)),
			// AppURL has already added BasePath to a local logo, so the base passed
			// here is the bare site url: carrying it twice points every og:image
			// under -base-path at /cairn/cairn/… and 404s.
			OGImage:   ogImage(cfg.Site.URL, AppURL(cfg.Site.Logo.Light)),
			Noindex:   cfg.Noindex(),
			AboutHash: aboutHash(cfg.Site.About),
			Style:     template.CSS(themeStyle(cfg)),
			CustomCSS: cfg.CustomCSS,
			Locales:   cfg.Site.Locales,
			// Leave itself is per page, set once the links are known.
			LeaveBlank: cfg.Site.ServiceLinks.NewTab,
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
				LeaveTitle:        cfg.Str(loc, "leave.title"),
				LeaveBody:         cfg.Str(loc, "leave.body"),
				LeaveCopy:         cfg.Str(loc, "leave.copy"),
				LeaveCopied:       cfg.Str(loc, "leave.copied"),
				LeaveGo:           cfg.Str(loc, "leave.go"),
				LeaveStay:         cfg.Str(loc, "leave.stay"),
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
					URL:       s.URL,
					Icon:      AppURL(config.IconURL(cfg, s.Icon.Light)),
					IconClass: themed.classFor(cfg, s.Icon),
					Name:      tr(s.Name, loc, def),
					Desc:      tr(s.Desc, loc, def),
					Tags:      strings.Join(s.Tags, " "),
					Off:       s.State.Disables(),
				}
				if s.State != config.StateNone {
					card.State = string(s.State)
					card.StateLabel = cfg.Str(loc, "state."+string(s.State))
				}
				// A disabling state means nothing is monitored, so there is no slot
				// for a pill to be swapped into.
				if !card.Off {
					card.Status = statusOf(cfg, statuses, s.ID)
					card.StatusID = statusSlot(cfg, s.ID)
					card.StatusLabel, card.StatusHref, card.StatusA11y = statusMeta(cfg, loc, card.Status, s, statuses[s.ID].Key)
				}
				if len(s.Details) > 0 || len(s.Images) > 0 {
					card.MoreHref = BasePath + "/" + loc + "/" + s.ID + "/"
					card.MoreA11y = s.Name.Get(loc, def) + ", " + cfg.Str(loc, "card.more")
				}
				if s.Selfhosted != nil {
					if *s.Selfhosted {
						card.HostKind, card.HostLabel = "self", cfg.Str(loc, "host.self")
						card.HostHref, card.HostBlank = hostTarget(cfg, loc, cfg.Site.HostingFlag.Self)
					} else {
						card.HostKind, card.HostLabel = "external", cfg.Str(loc, "host.external")
						card.HostHref, card.HostBlank = hostTarget(cfg, loc, cfg.Site.HostingFlag.External)
					}
				}
				// After the flag: the confirm scope reads HostKind. A disabling
				// state has no link at all, so neither key can mean anything.
				if !card.Off {
					card.Blank, card.Leave = serviceLink(cfg, s.URL, card.HostKind)
					hv.Leave = hv.Leave || card.Leave
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
					pageView:  base,
					Name:      tr(s.Name, loc, def),
					Desc:      tr(s.Desc, loc, def),
					Icon:      AppURL(config.IconURL(cfg, s.Icon.Light)),
					IconClass: themed.classFor(cfg, s.Icon),
					URL:       s.URL,
					Body:      proseOf(tr(s.Details, loc, def), mdCtx{media: media}),
					Off:       s.State.Disables(),
				}
				if s.State != config.StateNone {
					dv.State = string(s.State)
					dv.StateLabel = cfg.Str(loc, "state."+string(s.State))
				}
				if !dv.Off {
					dv.Status = statusOf(cfg, statuses, s.ID)
					dv.StatusID = statusSlot(cfg, s.ID)
				}
				for _, img := range s.Images {
					url, w, h := media(img.Src)
					dv.Images = append(dv.Images, imageView{Src: url, Caption: tr(img.Caption, loc, def), W: w, H: h})
				}
				if !dv.Off {
					dv.StatusLabel, dv.StatusHref, dv.StatusA11y = statusMeta(cfg, loc, dv.Status, s, statuses[s.ID].Key)
					// The detail page has to reach the same verdict as the card for
					// the same service, or a visitor meets the dialog on one route
					// and not the other.
					dv.Blank, dv.Leave = serviceLink(cfg, s.URL, hostKindOf(s))
					dv.pageView.Leave = dv.Leave
				}
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

const defaultTouchIcon = "/static/touch-icon.png"

// widestSize reads the largest width out of a manifest `sizes` value, which may
// list several ("48x48 96x96") or be "any". Anything unreadable is 0, so it
// loses to any real number without needing a separate case.
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
// iOS never reads the manifest for this: it reads the apple-touch-icon link and
// nothing else. An operator who listed their own icons has to be honoured here
// too, or their list would take effect on Android while an iPhone home screen
// showed cairn's mark. The largest is picked, since iOS downsamples and only a
// too-small icon shows.
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
	if strings.HasSuffix(strings.ToLower(cfg.Site.Favicon.Light), ".png") {
		return cfg.Site.Favicon.Light
	}
	return defaultTouchIcon
}

// AppIcon is one entry of the web app manifest's icon list. Everything but the
// source is omitted when unknown, rather than filled with a guess.
type AppIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes,omitempty"`
	Type    string `json:"type,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

// iconType is empty for a reference cairn does not recognise, and the manifest
// entry then omits the field rather than asserting a format.
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
// Every field is either true or absent. Sizes are never invented: an entry whose
// size cairn cannot establish carries no `sizes`, which the manifest spec
// allows, and the browser decides.
//
// Three cases, in order:
//
//   - The operator listed `icons` themselves. That wins outright, sizes and all,
//     since they are the only one who can know them.
//   - They set a `favicon`. An svg is scalable, so it is declared "any"; a raster
//     in the mounted assets dir is measured at load; a raster behind a URL is
//     offered with no size, since measuring it means an outbound request and
//     cairn makes none.
//   - Neither. cairn's own set, which ships the 192 and the 512 that Chromium
//     insists on before it will offer to install a site, both declared
//     "any maskable": a separate padded maskable file exists for full-bleed
//     icons that lose their edges to Android's crop, and this mark is a narrow
//     stack that clears that circle with room to spare.
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

	if fav := cfg.Site.Favicon.Light; fav != "" {
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

// IsRaster reports a format a link previewer can display. svg is out: og:image
// with a vector is ignored by most platforms, so a vector logo means no preview
// picture at all, and -check says so rather than leaving it to be discovered on
// a shared link.
func IsRaster(path string) bool {
	l := strings.ToLower(path)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		if strings.HasSuffix(l, ext) {
			return true
		}
	}
	return false
}

// absBase is the public origin every self-referencing link is built from. It
// stays empty without a site url: BasePath alone is a truthy "/cairn" in the
// template, and canonical, og:url and hreflang would go out relative instead of
// not at all.
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

// mediaURL resolves a bare image name against the /media/ route, which serves
// the config dir's media/ folder. URLs and absolute paths pass through, same
// convention as icons.
func mediaURL(src string) string {
	if config.IsURLOrAbs(src) {
		return src
	}
	return "/media/" + src
}

// Version is what the footer shows when show_version is on. Package main carries
// the -ldflags stamp and assigns it here at startup.
var Version = "dev"

// The repository the credit and the version stamp point at.
const repoURL = "https://github.com/MorganKryze/cairn"

var (
	semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// versionInfo turns the build stamp into a footer label and a link: a tagged
// release points at its release notes, a build off main at the commit it was cut
// from, and anything else (a local `go build`, which stamps nothing) is named
// but not linked, there being no public page for it.
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
