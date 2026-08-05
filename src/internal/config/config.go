package config

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// LString is a string translated per locale. A plain YAML string decodes as
// the value for every locale.
type LString map[string]string

func (l *LString) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = LString{"": n.Value}
		return nil
	}
	var m map[string]string
	if err := n.Decode(&m); err != nil {
		return fmt.Errorf("line %d: expected one plain text or a per-locale mapping like {fr: …, en: …}", n.Line)
	}
	// A key that is not a locale code is nearly always the flow-mapping
	// comma trap: {fr: Une phrase, avec virgule.} parses as two keys.
	for k := range m {
		if !localeRe.MatchString(k) {
			return fmt.Errorf("line %d: %q is not a locale code; an unquoted comma splits a {…} entry in two, so quote the text or write one locale per line", n.Line, k)
		}
	}
	*l = m
	return nil
}

// Get is the wording alone, for everything that only prints it. It is written
// in terms of GetLocale so there is one lookup order rather than two that can
// drift apart.
func (l LString) Get(locale, fallback string) string {
	s, _ := l.GetLocale(locale, fallback)
	return s
}

// GetLocale resolves a translated field and reports the locale the wording
// actually came from.
//
// That second half is what lets a page be honest about a fallback. A field
// with no translation for the page's locale still renders, in whichever
// language the config does have, and nothing used to say so: the page then
// asserts through <html lang> that the sentence is in a language it is not. A
// screen reader reads it in the wrong voice, and an Arabic fallback inside a
// left-to-right page is laid out the wrong way round, which moves its
// punctuation to the wrong end of the line.
//
// The order is unchanged: the exact locale, the untranslated form, the site
// default, then anything at all in key order so a half-written config still
// renders something.
//
// The untranslated form is the plain YAML string, kept under "". It reports
// the locale that was asked for rather than a language of its own: one string
// written for every locale is the operator saying it serves them all, which is
// what a brand name is, and calling that a fallback would mark most fields of
// most sites.
func (l LString) GetLocale(locale, fallback string) (text, from string) {
	if s := l[locale]; s != "" {
		return s, locale
	}
	if s := l[""]; s != "" {
		return s, locale
	}
	if s := l[fallback]; s != "" {
		return s, fallback
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if l[k] != "" {
			return l[k], k
		}
	}
	return "", ""
}

type FooterLink struct {
	Label LString `yaml:"label"`
	URL   string  `yaml:"url"`
	Icon  string  `yaml:"icon"` // links only: a built-in glyph name, URL or /assets path; a footer entry carrying one is refused
}

// SitePage is a page cairn serves itself (legal notice, privacy…), linked
// automatically in the footer after the manual entries. body is an optional
// intro; sections add titled blocks below it.
type SitePage struct {
	ID       string        `yaml:"id"`
	Title    LString       `yaml:"title"`
	Body     LString       `yaml:"body"`
	Sections []PageSection `yaml:"sections"`
}

type PageSection struct {
	Title LString `yaml:"title"`
	Body  LString `yaml:"body"`
}

// StatusMap reads a flat array of objects out of any JSON status API: where
// the array is, which field of each element holds the service name and which
// holds its state, then the values that mean each level. A path is a dotted
// walk and nothing more, because anything cleverer is a query language, and a
// query language in YAML is a second product.
//
// The value lists are allow-lists: a state in none of them reads as down, so a
// vendor that adds a word next year cannot make a broken service look green.
// The exception is unknown, which is read first and means the monitor said
// nothing at all about that service.
type StatusMap struct {
	List        string   `yaml:"list"`
	Key         string   `yaml:"key"`
	State       string   `yaml:"state"`
	Up          []string `yaml:"up"`
	Degraded    []string `yaml:"degraded"`
	Maintenance []string `yaml:"maintenance"`
	// Unknown are the values that mean nobody is checking, a monitor paused or
	// waiting for its first verdict. Those get no pill rather than a red one.
	Unknown []string `yaml:"unknown"`
}

// SiteIcon is one home-screen icon an operator supplies themselves. cairn
// cannot resize an image without pulling in a scaler, so an operator who wants
// the full set names the files and their sizes; nothing here is guessed.
type SiteIcon struct {
	Src     string `yaml:"src"`
	Sizes   string `yaml:"sizes"`   // "512x512", or "any" for an svg
	Purpose string `yaml:"purpose"` // optional: any, maskable, monochrome
}

type Site struct {
	Title   LString    `yaml:"title"`
	Tagline LString    `yaml:"tagline"`
	URL     string     `yaml:"url"` // public base URL; enables canonical/hreflang
	Logo    ThemedRef  `yaml:"logo"`
	Favicon ThemedRef  `yaml:"favicon"` // tab icon; URL or /assets path, cairn's own by default
	Icons   []SiteIcon `yaml:"icons"`   // home-screen set; overrides everything derived from favicon
	Index   *bool      `yaml:"index"`   // nil means true; false asks search engines to stay away
	Locales []string   `yaml:"locales"`
	Theme   struct {
		Accent string `yaml:"accent"`
		Font   struct {
			Family string `yaml:"family"`
			File   string `yaml:"file"`
		} `yaml:"font"`
	} `yaml:"theme"`
	About   LString            `yaml:"about"`
	Links   []FooterLink       `yaml:"links"`
	Footer  []FooterLink       `yaml:"footer"`
	Pages   []SitePage         `yaml:"pages"`
	Credit  *bool              `yaml:"credit"`       // the footer "powered by cairn"; nil means true
	ShowVer bool               `yaml:"show_version"` // print the running version beside the credit; off by default
	Strings map[string]LString `yaml:"strings"`
	// Security fills /.well-known/security.txt. Contact is the only field an
	// operator writes: Expires, Canonical and Preferred-Languages are cairn's
	// to compute, which is the whole reason to serve the file rather than let
	// one go stale in a folder.
	Security struct {
		Contact    string `yaml:"contact"`    // a URI: mailto:…, https://… or tel:…
		Policy     string `yaml:"policy"`     // where the disclosure policy lives
		Encryption string `yaml:"encryption"` // where the public key lives
	} `yaml:"security"`
	// HostingFlag is where the self-hosted and external flags lead, when the
	// operator wants them to lead anywhere. Each value is a page id cairn
	// serves, an absolute path, or a URL; a page id resolves in the language
	// the visitor is reading, the way the footer links to those pages already.
	//
	// Empty leaves the flags exactly as they were, plain text on the card.
	HostingFlag struct {
		Self     string `yaml:"self"`
		External string `yaml:"external"`
	} `yaml:"hosting_flag"`
	// ServiceLinks is how the two links that lead to a service behave: the
	// name on a card, and the button on a detail page. Both are off, so a
	// site that says nothing keeps exactly the page it has today.
	//
	// The block is `service_links` and not `links`, which is what the feature
	// request called it: `links` has been the header link list since long
	// before this, and a key cannot be a sequence and a mapping at once.
	//
	// Neither key touches a link cairn serves itself. A "Learn more" page, the
	// language switcher, a footer entry: those stay where they are, on the
	// same off-site test the hosting flag already applies. What is being
	// described is leaving the site, not following a link.
	ServiceLinks struct {
		NewTab  bool         `yaml:"new_tab"`
		Confirm ConfirmScope `yaml:"confirm"`
	} `yaml:"service_links"`
	Status struct {
		Gatus string `yaml:"gatus"`
		// Provider names the monitor behind the address, and URL is that
		// address for a monitor that is not Gatus. Gatus keeps its own key
		// because it had one first: every site running today says gatus: and
		// has to keep meaning exactly what it meant. The two spellings resolve
		// through StatusProvider and StatusAddress, and nothing else reads
		// these three fields.
		Provider string `yaml:"provider"`
		URL      string `yaml:"url"`
		// Map tells cairn how to read somebody else's status document, and
		// TokenFile names the file holding the credential for it. Its twin in
		// the status package is status.Mapping: this package knows about no
		// other, so server/watch.go copies one into the other the way it
		// already copies the address and the trust settings.
		Map StatusMap `yaml:"map"`
		// TokenFile is a path, never a token. A secret written in site.yaml is
		// a secret in a config repository; every platform that has secrets
		// delivers them as a mounted file, which is the shape status.ca takes
		// already.
		TokenFile   string `yaml:"token_file"`
		TokenScheme string `yaml:"token_scheme"` // default Bearer; Statuspage wants OAuth
		// Slug is the published status page a Kuma instance serves statuses
		// for. Kuma has no endpoint listing every monitor, only one per status
		// page, so the slug is the address as much as the URL is.
		Slug     string `yaml:"slug"`
		Page     string `yaml:"page"`
		Interval string `yaml:"interval"`
		// Linked nil means true: the pills link to the status page. false
		// makes them display-only, for a Gatus a visitor cannot reach.
		Linked *bool `yaml:"linked"`
		// Insecure stops cairn verifying the certificate Gatus presents, for
		// an internal instance whose authority nothing public signed. A hole
		// the operator opens deliberately, so cairn says so out loud rather
		// than only obeying: once at startup, and on every -check.
		Insecure bool `yaml:"insecure"`
		// CA is a PEM bundle cairn adds to the system roots for that one
		// connection: an http(s) URL, or an /assets/… path in the mounted
		// directory. It is the answer to the same problem Insecure is, and the
		// opposite answer: this one verifies rather than stops checking.
		//
		// A URL is fetched, which is why http is allowed here and why saying
		// so matters. Over http, whatever is on the path to that address
		// decides what cairn trusts for the poll, which lands in the same
		// place Insecure does. cairn says which of the two it is at startup
		// and on every -check.
		CA string `yaml:"ca"`
	} `yaml:"status"`
}

type Service struct {
	ID       string         `yaml:"id"`
	URL      string         `yaml:"url"`
	Category string         `yaml:"category"`
	Icon     ThemedRef      `yaml:"icon"`
	Name     LString        `yaml:"name"`
	Desc     LString        `yaml:"desc"`
	Details  LString        `yaml:"details"`
	Images   []ServiceImage `yaml:"images"`
	Tags     []string       `yaml:"tags"`
	// Selfhosted flags where the service runs: true self-hosted, false hosted
	// elsewhere, nil no flag at all.
	Selfhosted *bool `yaml:"selfhosted"`
	// State is where the service stands, declared rather than measured. Empty
	// is the whole of what cairn rendered before this key existed.
	State State `yaml:"state"`
}

// ServiceImage is a preview shown on the detail page. A plain YAML string is
// a src without caption. Bare names resolve to files in the config dir's
// media/ folder, served at /media/; URLs and absolute paths pass through.
type ServiceImage struct {
	Src     string  `yaml:"src"`
	Caption LString `yaml:"caption"`
}

// imageEntry is ServiceImage without its UnmarshalYAML, which decoding the
// mapping form would otherwise re-enter forever. It sits at package level
// rather than inside the method so an unknown key in an image entry can be
// told what an image entry accepts; yaml names this type in its error.
type imageEntry ServiceImage

func (i *ServiceImage) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*i = ServiceImage{Src: n.Value}
		return nil
	}
	var v imageEntry
	if err := strictDecode(n, &v); err != nil {
		return err
	}
	*i = ServiceImage(v)
	return nil
}

type CategoryMeta struct {
	ID    string  `yaml:"id"`
	Name  LString `yaml:"name"`
	Order *int    `yaml:"order"`
}

type Category struct {
	ID       string
	Name     LString
	Order    *int
	Services []Service
}

type Config struct {
	Site       Site
	Categories []Category
	CustomCSS  bool
	MediaDims  map[string][2]int // media/ file -> intrinsic width, height
	LocalIcons map[string]string // slug -> /assets/icons/… when self-hosted
	// FaviconDims is the operator favicon's real size, when it is a raster
	// sitting in the mounted assets dir. Zero for an svg, for a remote URL,
	// and when no favicon is set: the manifest then says nothing about size
	// rather than claiming one, which is what it used to do.
	FaviconDims [2]int
}

func (c *Config) DefaultLocale() string { return c.Site.Locales[0] }

// StatusProviders lists the monitors status.provider accepts, sorted.
//
// This package imports nothing of cairn's, which is the rule the dependency
// graph is built on, so the list cannot come from the poller that owns it. It
// is kept in step by a test over there, where both are in reach: a name in one
// list and not the other is either a config error nobody can fix or a config
// that loads clean and then fails every poll.
func StatusProviders() []string { return []string{"gatus", "json", "kuma"} }

// StatusAddress is the monitor cairn polls, whichever key named it, and empty
// when the site has no status at all. Everything that used to ask whether
// status.gatus was set asks this instead.
func (s *Site) StatusAddress() string {
	if s.Status.Gatus != "" {
		return s.Status.Gatus
	}
	return s.Status.URL
}

// StatusProvider is which monitor that address is. status.gatus implies it, so
// a config written before providers existed resolves to gatus without saying
// so, and empty means the site polls nothing.
func (s *Site) StatusProvider() string {
	if s.StatusAddress() == "" && s.Status.Provider == "" {
		return ""
	}
	if s.Status.Provider != "" {
		return s.Status.Provider
	}
	return "gatus"
}

func (c *Config) StatusInterval() time.Duration {
	if d, err := time.ParseDuration(c.Site.Status.Interval); err == nil && d >= 5*time.Second {
		return d
	}
	return 60 * time.Second
}

// Noindex reports whether the site asks search engines to stay away.
func (c *Config) Noindex() bool { return c.Site.Index != nil && !*c.Site.Index }

// StatusLinked reports whether a status pill is a link to the status page.
// It is, unless the operator says otherwise: cairn cannot tell whether the
// Gatus it polls is one a visitor can reach, so only they can.
func (c *Config) StatusLinked() bool {
	return c.Site.Status.Linked == nil || *c.Site.Status.Linked
}

// Str resolves a UI string: site.yaml override (only for the locales it
// defines), then the locale's built-in set, its base language (pt-BR finds
// pt), English, then the key.
func (c *Config) Str(locale, key string) string {
	if ls, ok := c.Site.Strings[key]; ok {
		for _, k := range []string{locale, ""} {
			if s := ls[k]; s != "" {
				return s
			}
		}
	}
	base, _, _ := strings.Cut(locale, "-")
	for _, l := range []string{locale, strings.ToLower(base), "en"} {
		if s := builtinStrings[l][key]; s != "" {
			return s
		}
	}
	return key
}

func (c *Config) CategoryName(cat Category, locale string) string {
	if len(cat.Name) > 0 {
		if s := cat.Name.Get(locale, c.DefaultLocale()); s != "" {
			return s
		}
	}
	if cat.ID == "other" {
		return c.Str(locale, "cat.other")
	}
	name := strings.ReplaceAll(cat.ID, "-", " ")
	r := []rune(name)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

var (
	// CSS hex colours are 3, 4, 6 or 8 digits. 5 and 7 used to pass, and an
	// invalid custom property makes every var(--accent) declaration invalid at
	// computed-value time: the accent, and every focus ring drawn with it,
	// silently disappears.
	accentRe = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	// A CSS font-family list goes verbatim into the page's inline <style>,
	// so it is kept to the characters a family stack is made of: letters,
	// digits, spaces, quotes and the punctuation between family names. A
	// semicolon, a brace or a newline could break out of the declaration, and
	// refusing them is what lets the value be inlined without escaping.
	//
	// Letter means letter in any script. A font family is a name, and a name
	// is written in the language it belongs to: Époque, 思源黑体, الجزيرة. An
	// ASCII-only rule refuses those, and refusing a family refuses the config
	// it is in, so a site would lose every page to the getting-started one
	// over a font. None of that widens what can end a declaration.
	fontFamilyRe = regexp.MustCompile(`^[\p{L}\p{N} '",._@+-]+$`)
	// theme.font.file lands in the same inline stylesheet, as the url() of the
	// @font-face, and needs the same kind of guard for the same reason: a path
	// can pass every check about where it points and still carry the sequence
	// that ends a style element. This is the shape of a filename.
	fontFileRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/._-]*$`)
	localeRe   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)
	idRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	// An icon slug: what dashboard-icons publishes, and the only shape that can
	// safely become both a filename and a URL segment.
	slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	// A manifest icon size: one or more "WxH", or "any" for a scalable one.
	iconSizesRe = regexp.MustCompile(`^(any|[0-9]+x[0-9]+( [0-9]+x[0-9]+)*)$`)
	// Any URI with a scheme, since security.txt takes mailto:, https:// and
	// tel: alike. Checking the scheme is what catches a bare email address,
	// which is the mistake this field invites.
	uriRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:[^\s]+$`)
)

// isHTTPURL reports an http(s) URL; IsURLOrAbs also accepts a root-absolute
// /path. Both gate the "is this a link we pass through" checks scattered
// across config and render.
func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// IsLocalPath reports a root-absolute path that stays on this site.
//
// "//cdn.example.org/x" is not one, though it starts with a slash: a browser
// reads it as a URL on another origin. The backslash form counts as external
// too, because browsers normalise it to the same thing. Every place that has
// to prefix a base path or a site URL needs this distinction rather than a
// bare leading slash, or a protocol-relative logo comes out as
// "/cairn//cdn.example.org/x", which resolves nowhere.
func IsLocalPath(s string) bool {
	return strings.HasPrefix(s, "/") &&
		!strings.HasPrefix(s, "//") && !strings.HasPrefix(s, `/\`)
}

// isExternalURL reports a link that leaves this site: an explicit scheme, or
// the protocol-relative form.
func isExternalURL(s string) bool {
	return isHTTPURL(s) || strings.HasPrefix(s, "//") || strings.HasPrefix(s, `/\`)
}

// IsURLOrAbs gates "is this a link we pass through rather than a slug we
// resolve". It accepts exactly what it always did; it just no longer confuses
// the two kinds of leading slash on the way.
func IsURLOrAbs(s string) bool {
	return isExternalURL(s) || IsLocalPath(s)
}

// mediaRef maps an images: entry to the file it names under media/, and
// reports whether it names one at all.
//
// A file in media/ has two documented spellings, the bare name and the
// /media/… path it is served at, and they have to mean the same thing here:
// the bare one was stat'd and the absolute one was not, so the same missing
// file refused to boot written one way and rendered a 404 written the other.
//
// Everything else stays out of it, because it is not this directory's to
// serve: a URL, /assets/… from the mounted assets dir, any other absolute
// path. Note that a value starting with "/media/" cannot be the
// protocol-relative "//host/x" or its "/\host/x" twin, so the prefix already
// draws the line IsLocalPath exists to draw. ".." cannot arrive either;
// parseServices refuses it in every images entry before this runs.
func mediaRef(src string) (rel string, ours bool) {
	if !IsURLOrAbs(src) {
		return src, true
	}
	return strings.CutPrefix(src, "/media/")
}

// FontRef maps theme.font.file to the file under the config directory's
// fonts/ folder, and reports whether the value names one at all. The served
// URL is "/fonts/" + rel, which is why the two spellings that are already a
// path mean the file it leads to.
//
// A font file has three documented spellings that have to mean the same
// thing: the bare name "custom-font.woff2", the config-relative
// "fonts/custom-font.woff2", and the "/fonts/custom-font.woff2" path it is
// served at. Everything else -- a URL, an absolute path outside fonts/ , a
// "../" climb -- comes back not-ok, and the caller answers with its own
// message. The backslash form counts as a climb because a browser normalises
// it to the same thing, the rule IsLocalPath already draws.
func FontRef(file string) (rel string, ok bool) {
	if file == "" {
		return "", false
	}
	file = strings.TrimPrefix(file, "/fonts/")
	file = strings.TrimPrefix(file, "fonts/")
	if file == "" || strings.Contains(file, "..") || strings.ContainsAny(file, `\`) ||
		strings.HasPrefix(file, "/") || uriRe.MatchString(file) {
		return "", false
	}
	return file, true
}

// FirstFontFamily is the first entry of a CSS font-family list, the name an
// @font-face can declare. A list like "Inter, system-ui, sans-serif" names
// the custom file first, and the @font-face has to use exactly that name for
// the browser to connect the two; splitting on a comma a quote belongs to
// would cut a name like "My, Font" in half.
func FirstFontFamily(family string) string {
	var b strings.Builder
	quote := rune(0)
	for _, r := range family {
		switch {
		case quote != 0:
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			b.WriteRune(r)
			quote = r
		case r == ',':
			return strings.TrimSpace(b.String())
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// FontFormat maps a font file's extension to the format hint its @font-face
// src carries. Empty for a file cairn has no label for, which validation
// refuses before it ever reaches a page.
func FontFormat(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".woff2":
		return "woff2"
	case ".woff":
		return "woff"
	case ".ttf":
		return "truetype"
	case ".otf":
		return "opentype"
	}
	return ""
}

// FontFaceName is the name the @font-face for theme.font.file declares: the
// first family of theme.font.family, quoted if it is not already. The
// @font-face and the --font-body override have to name the same font, so both
// come from the one value.
func FontFaceName(family string) string {
	return quotedFontName(FirstFontFamily(family))
}

func Load(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s (mount your yaml files there): %w", dir, err)
	}

	site := defaultSite()

	var services []Service
	var metas []CategoryMeta
	definedIn := map[string]string{}
	foundServices := false

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("config: %s: %w", name, err)
		}
		switch strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml") {
		case "site":
			site, err = parseSite(name, data)
			if err != nil {
				return nil, err
			}
		case "categories":
			metas, err = parseCategories(name, data)
			if err != nil {
				return nil, err
			}
		default:
			svcs, err := parseServices(name, data)
			if err != nil {
				return nil, err
			}
			for _, s := range svcs {
				if prev, dup := definedIn[s.ID]; dup {
					return nil, fmt.Errorf("config: %s: duplicate service id %q (already defined in %s)", name, s.ID, prev)
				}
				definedIn[s.ID] = name
			}
			services = append(services, svcs...)
			foundServices = true
		}
	}

	if !foundServices {
		return nil, fmt.Errorf("config: no services file in %s (expected at least one yaml list of services, e.g. services.yaml)", dir)
	}
	for _, s := range services {
		for _, img := range s.Images {
			rel, ours := mediaRef(img.Src)
			if !ours {
				continue
			}
			path := filepath.Join(dir, "media", filepath.FromSlash(rel))
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				return nil, fmt.Errorf("config: service %q: image %q not found (expected a file at %s, a URL or an absolute path)", s.ID, img.Src, path)
			}
		}
	}
	if err := validateSite(&site, definedIn); err != nil {
		return nil, err
	}

	cfg := &Config{
		Site:       site,
		Categories: groupCategories(services, metas),
		MediaDims:  mediaDims(filepath.Join(dir, "media")),
		LocalIcons: localIcons(filepath.Join(AssetsPath, "icons")),
	}
	// The manifest and /favicon.ico have no theme to follow, so both read the
	// light one: it is the file a site with one favicon already names.
	cfg.FaviconDims = assetDims(site.Favicon.Light)
	if st, err := os.Stat(filepath.Join(dir, "custom.css")); err == nil && !st.IsDir() {
		cfg.CustomCSS = true
	}
	return cfg, nil
}

// AssetFile turns an /assets/… reference into the file it names in the mounted
// directory, and returns "" for anything that is not one: a URL, a bare slug,
// or a path that tries to climb out of the mount, which is the rule the file
// server applies too. It touches no disk, so it answers the same either way.
func AssetFile(ref string) string {
	rel, ok := strings.CutPrefix(ref, "/assets/")
	if !ok || rel == "" || strings.Contains(rel, "..") {
		return ""
	}
	return filepath.Join(AssetsPath, filepath.FromSlash(rel))
}

// assetDims measures a raster the operator dropped in the mounted assets dir,
// so the manifest can state its real size instead of guessing one.
//
// Anything it cannot open comes back zero, on purpose: a remote URL would need
// an outbound request, which cairn does not make, and an svg has no intrinsic
// size to report. Both cases are handled by saying nothing rather than wrong.
func assetDims(ref string) [2]int {
	p := AssetFile(ref)
	if p == "" {
		return [2]int{}
	}
	f, err := os.Open(p)
	if err != nil {
		return [2]int{}
	}
	defer func() { _ = f.Close() }()
	c, _, err := image.DecodeConfig(f)
	if err != nil {
		return [2]int{}
	}
	return [2]int{c.Width, c.Height}
}

// mediaDims reads the intrinsic size of every image in media/, so the pages
// can carry width/height attributes and never shift while loading.
func mediaDims(dir string) map[string][2]int {
	out := map[string][2]int{}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		c, _, err := image.DecodeConfig(f)
		_ = f.Close()
		if err == nil {
			if rel, rerr := filepath.Rel(dir, p); rerr == nil {
				out[filepath.ToSlash(rel)] = [2]int{c.Width, c.Height}
			}
		}
		return nil
	})
	return out
}

// strictDecode decodes a yaml node rejecting unknown fields, which
// node.Decode cannot do on its own; a typo must be an error, not a no-op.
func strictDecode(n *yaml.Node, out any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s", yamlErr(err))
	}
	return nil
}

// defaultSite is what a directory with no site.yaml means, and equally the
// base a site.yaml decodes onto: every key in the file is optional, so the
// two have to agree. One function so they cannot stop agreeing.
func defaultSite() Site {
	s := Site{Title: LString{"": "cairn"}, Locales: []string{"en"}}
	s.Theme.Accent = "#247b7b"
	return s
}

// parseSite decodes site.yaml over those defaults. An empty file is a legal
// site.yaml, so io.EOF means "nothing set", not "broken".
func parseSite(file string, data []byte) (Site, error) {
	site := defaultSite()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&site); err != nil && !errors.Is(err, io.EOF) {
		return Site{}, fmt.Errorf("config: %s: %s", file, yamlErr(err))
	}
	return site, nil
}

func parseCategories(file string, data []byte) ([]CategoryMeta, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("config: %s: %v", file, err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("config: %s line %d: expected a list of categories (- id: …)", file, root.Line)
	}
	var out []CategoryMeta
	seen := map[string]bool{}
	for _, item := range root.Content {
		var m CategoryMeta
		if err := strictDecode(item, &m); err != nil {
			return nil, fmt.Errorf("config: %s line %d: %s", file, item.Line, entryErr(err))
		}
		if m.ID == "" {
			return nil, fmt.Errorf("config: %s line %d: category missing id (expected: id: documents)", file, item.Line)
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("config: %s line %d: duplicate category id %q", file, item.Line, m.ID)
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	return out, nil
}

func parseServices(file string, data []byte) ([]Service, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("config: %s: %v", file, err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("config: %s line %d: expected a list of services (- id: …)", file, root.Line)
	}
	var out []Service
	for _, item := range root.Content {
		var s Service
		if err := strictDecode(item, &s); err != nil {
			return nil, fmt.Errorf("config: %s line %d: %s", file, item.Line, entryErr(err))
		}
		switch {
		case s.ID == "":
			return nil, fmt.Errorf("config: %s line %d: service missing id (expected: id: my-tool)", file, item.Line)
		case !idRe.MatchString(s.ID):
			return nil, fmt.Errorf("config: %s line %d: invalid id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. my-tool)", file, item.Line, s.ID)
		// The two disabling states have no destination worth naming, so the url
		// stops being required for them and only for them. Retiring a service
		// has to be adding a line rather than deleting one, so a url left
		// beside `state: retired` is kept and simply not linked.
		case !IsURLOrAbs(s.URL) && !s.State.Disables():
			return nil, fmt.Errorf("config: %s line %d: service %q missing or invalid url (expected: url: https://…, or state: soon / state: retired, which do not need one)", file, item.Line, s.ID)
		case len(s.Name) == 0:
			return nil, fmt.Errorf("config: %s line %d: service %q missing name (expected: name: My tool  or  name: {fr: …, en: …})", file, item.Line, s.ID)
			// A slug becomes a path segment on the icon CDN and a filename in the
			// script -emit-icons writes, which the docs tell you to pipe into sh.
			// Anything outside this shape could never resolve upstream anyway, so
			// refusing it costs nothing and closes the shell off entirely.
		}
		// Both halves of a themed icon are held to the same shape: each one
		// becomes a path segment on the CDN and a filename in the -emit-icons
		// script all the same.
		for _, ic := range s.Icon.Refs() {
			if !IsURLOrAbs(ic) && !slugRe.MatchString(ic) {
				return nil, fmt.Errorf("config: %s line %d: service %q: icon %q is not a slug, a URL or an /assets path (a slug is lowercase letters, digits, dashes, e.g. hedgedoc)", file, item.Line, s.ID, ic)
			}
		}
		// The dark icon is written into the page's stylesheet, so it answers
		// to the same rule the dark logo does. A slug cannot carry either
		// character; a URL or a path can.
		if s.Icon.Themed() && strings.ContainsAny(s.Icon.Dark, "<>") {
			return nil, fmt.Errorf("config: %s line %d: service %q: icon.dark %q carries %q or %q, and the dark icon is written into the page's stylesheet where nothing escapes it (expected a slug like github-light, or /assets/github-white.svg)", file, item.Line, s.ID, s.Icon.Dark, "<", ">")
		}
		for _, img := range s.Images {
			switch {
			case img.Src == "":
				return nil, fmt.Errorf("config: %s line %d: service %q: every images entry needs a src (expected: - screen.png  or  - {src: screen.png, caption: …})", file, item.Line, s.ID)
			case strings.Contains(img.Src, ".."):
				return nil, fmt.Errorf("config: %s line %d: service %q: image src %q must not contain %q", file, item.Line, s.ID, img.Src, "..")
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// groupCategories derives categories from the services and decorates them
// with categories.yaml metadata. Explicitly ordered categories come first,
// the rest are alphabetical with the implicit "other" bucket last. Services
// keep the order of their file; files merge in name order. Categories
// declared without services are not rendered.
func groupCategories(services []Service, metas []CategoryMeta) []Category {
	grouped := map[string][]Service{}
	for _, s := range services {
		id := s.Category
		if id == "" {
			id = "other"
		}
		grouped[id] = append(grouped[id], s)
	}
	meta := map[string]CategoryMeta{}
	for _, m := range metas {
		meta[m.ID] = m
	}
	cats := make([]Category, 0, len(grouped))
	for id, svcs := range grouped {
		cats = append(cats, Category{ID: id, Name: meta[id].Name, Order: meta[id].Order, Services: svcs})
	}
	rank := func(c Category) int {
		switch {
		case c.Order != nil:
			return 0
		case c.ID == "other":
			return 2
		default:
			return 1
		}
	}
	sort.Slice(cats, func(i, j int) bool {
		a, b := cats[i], cats[j]
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		if a.Order != nil && b.Order != nil && *a.Order != *b.Order {
			return *a.Order < *b.Order
		}
		return a.ID < b.ID
	})
	return cats
}

func CountServices(cfg *Config) int {
	n := 0
	for _, c := range cfg.Categories {
		n += len(c.Services)
	}
	return n
}
