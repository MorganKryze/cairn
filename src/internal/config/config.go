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

func (l LString) Get(locale, fallback string) string {
	for _, k := range []string{locale, "", fallback} {
		if s := l[k]; s != "" {
			return s
		}
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if l[k] != "" {
			return l[k]
		}
	}
	return ""
}

type FooterLink struct {
	Label LString `yaml:"label"`
	URL   string  `yaml:"url"`
	Icon  string  `yaml:"icon"` // header links only: a built-in glyph name, URL or /assets path
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
	Logo    string     `yaml:"logo"`
	Favicon string     `yaml:"favicon"` // tab icon; URL or /assets path, cairn's own by default
	Icons   []SiteIcon `yaml:"icons"`   // home-screen set; overrides everything derived from favicon
	Index   *bool      `yaml:"index"`   // nil means true; false asks search engines to stay away
	Locales []string   `yaml:"locales"`
	Theme   struct {
		Accent string `yaml:"accent"`
	} `yaml:"theme"`
	About   LString            `yaml:"about"`
	Links   []FooterLink       `yaml:"links"`
	Footer  []FooterLink       `yaml:"footer"`
	Pages   []SitePage         `yaml:"pages"`
	Credit  *bool              `yaml:"credit"`       // the footer "powered by cairn"; nil means true
	ShowVer bool               `yaml:"show_version"` // print the running version beside the credit; off by default
	Strings map[string]LString `yaml:"strings"`
	Status  struct {
		Gatus    string `yaml:"gatus"`
		Page     string `yaml:"page"`
		Interval string `yaml:"interval"`
	} `yaml:"status"`
}

type Service struct {
	ID       string         `yaml:"id"`
	URL      string         `yaml:"url"`
	Category string         `yaml:"category"`
	Icon     string         `yaml:"icon"`
	Name     LString        `yaml:"name"`
	Desc     LString        `yaml:"desc"`
	Details  LString        `yaml:"details"`
	Images   []ServiceImage `yaml:"images"`
	Tags     []string       `yaml:"tags"`
	// Selfhosted flags where the service runs: true self-hosted, false hosted
	// elsewhere, nil no flag at all.
	Selfhosted *bool `yaml:"selfhosted"`
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

func (c *Config) StatusInterval() time.Duration {
	if d, err := time.ParseDuration(c.Site.Status.Interval); err == nil && d >= 5*time.Second {
		return d
	}
	return 60 * time.Second
}

// Noindex reports whether the site asks search engines to stay away.
func (c *Config) Noindex() bool { return c.Site.Index != nil && !*c.Site.Index }

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
	accentRe = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	localeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)
	idRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	// A manifest icon size: one or more "WxH", or "any" for a scalable one.
	iconSizesRe = regexp.MustCompile(`^(any|[0-9]+x[0-9]+( [0-9]+x[0-9]+)*)$`)
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
			src := img.Src
			if IsURLOrAbs(src) {
				continue
			}
			if st, err := os.Stat(filepath.Join(dir, "media", filepath.FromSlash(src))); err != nil || st.IsDir() {
				return nil, fmt.Errorf("config: service %q: image %q not found (expected a file at %s, a URL or an absolute path)", s.ID, src, filepath.Join(dir, "media", src))
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
	cfg.FaviconDims = assetDims(site.Favicon)
	if st, err := os.Stat(filepath.Join(dir, "custom.css")); err == nil && !st.IsDir() {
		cfg.CustomCSS = true
	}
	return cfg, nil
}

// assetDims measures a raster the operator dropped in the mounted assets dir,
// so the manifest can state its real size instead of guessing one.
//
// Anything it cannot open comes back zero, on purpose: a remote URL would need
// an outbound request, which cairn does not make, and an svg has no intrinsic
// size to report. Both cases are handled by saying nothing rather than wrong.
func assetDims(ref string) [2]int {
	const mount = "/assets/"
	if !strings.HasPrefix(ref, mount) {
		return [2]int{}
	}
	rel := filepath.FromSlash(strings.TrimPrefix(ref, mount))
	// Refuse to walk out of the mount, the same rule the file server applies.
	if rel == "" || strings.Contains(rel, "..") {
		return [2]int{}
	}
	f, err := os.Open(filepath.Join(AssetsPath, rel))
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
		case !IsURLOrAbs(s.URL):
			return nil, fmt.Errorf("config: %s line %d: service %q missing or invalid url (expected: url: https://…)", file, item.Line, s.ID)
		case len(s.Name) == 0:
			return nil, fmt.Errorf("config: %s line %d: service %q missing name (expected: name: My tool  or  name: {fr: …, en: …})", file, item.Line, s.ID)
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
