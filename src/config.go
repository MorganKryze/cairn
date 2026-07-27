package main

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

type Site struct {
	Title   LString  `yaml:"title"`
	Tagline LString  `yaml:"tagline"`
	URL     string   `yaml:"url"` // public base URL; enables canonical/hreflang
	Logo    string   `yaml:"logo"`
	Favicon string   `yaml:"favicon"` // tab icon; URL or /assets path, cairn's own by default
	Index   *bool    `yaml:"index"`   // nil means true; false asks search engines to stay away
	Locales []string `yaml:"locales"`
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

func (i *ServiceImage) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*i = ServiceImage{Src: n.Value}
		return nil
	}
	type imageEntry ServiceImage
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

func (c *Config) categoryName(cat Category, locale string) string {
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
)

// isHTTPURL reports an http(s) URL; isURLOrAbs also accepts a root-absolute
// /path. Both gate the "is this a link we pass through" checks scattered
// across config and render.
func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func isURLOrAbs(s string) bool {
	return isHTTPURL(s) || strings.HasPrefix(s, "/")
}

func loadConfig(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s (mount your yaml files there): %w", dir, err)
	}

	site := Site{Title: LString{"": "cairn"}, Locales: []string{"en"}}
	site.Theme.Accent = "#247b7b"

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
			dec := yaml.NewDecoder(bytes.NewReader(data))
			dec.KnownFields(true)
			if err := dec.Decode(&site); err != nil && !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("config: %s: %s (expected keys: title, tagline, url, logo, favicon, index, locales, theme.accent, about, links, footer, pages, credit, strings, status)", name, yamlErr(err))
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
			if isURLOrAbs(src) {
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
		LocalIcons: localIcons(filepath.Join(assetsPath, "icons")),
	}
	if st, err := os.Stat(filepath.Join(dir, "custom.css")); err == nil && !st.IsDir() {
		cfg.CustomCSS = true
	}
	return cfg, nil
}

// validateSite normalizes and checks the site.yaml fields (defaults, hex
// accent, URLs, links, pages). definedIn is the service-id -> file map, used
// to reject page ids that collide with a service id.
func validateSite(site *Site, definedIn map[string]string) error {
	if len(site.Locales) == 0 {
		site.Locales = []string{"en"}
	}
	for _, l := range site.Locales {
		if !localeRe.MatchString(l) {
			return fmt.Errorf("config: site.yaml: invalid locale %q (expected codes like fr, en, pt-BR)", l)
		}
	}
	if !accentRe.MatchString(site.Theme.Accent) {
		return fmt.Errorf("config: site.yaml: theme.accent %q is not a hex color (expected e.g. \"#247b7b\")", site.Theme.Accent)
	}
	if g := site.Status.Gatus; g != "" && !isHTTPURL(g) {
		return fmt.Errorf("config: site.yaml: status.gatus %q is not a URL (expected e.g. https://status.example.org)", g)
	}
	if p := site.Status.Page; p != "" && !isHTTPURL(p) {
		return fmt.Errorf("config: site.yaml: status.page %q is not a URL (expected e.g. https://status.example.org)", p)
	}
	if iv := site.Status.Interval; iv != "" {
		if d, err := time.ParseDuration(iv); err != nil || d < 5*time.Second {
			return fmt.Errorf("config: site.yaml: status.interval %q is not a duration of at least 5s (expected e.g. 60s)", iv)
		}
	}
	site.URL = strings.TrimSuffix(site.URL, "/")
	if u := site.URL; u != "" && !isHTTPURL(u) {
		return fmt.Errorf("config: site.yaml: url %q is not a URL (expected e.g. https://tools.example.org)", u)
	}
	for _, l := range site.Links {
		if len(l.Label) == 0 || l.URL == "" {
			return fmt.Errorf("config: site.yaml: every links entry needs label and url (expected: - {label: Wiki, url: https://…})")
		}
		if ic := l.Icon; ic != "" && !isURLOrAbs(ic) {
			if _, ok := linkGlyphs[ic]; !ok {
				return fmt.Errorf("config: site.yaml: links icon %q is not a built-in glyph (%s), a URL or an /assets path", ic, strings.Join(linkGlyphNames(), ", "))
			}
		}
	}
	pageIDs := map[string]bool{}
	for _, p := range site.Pages {
		switch {
		case !idRe.MatchString(p.ID):
			return fmt.Errorf("config: site.yaml: invalid page id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. legal)", p.ID)
		case len(p.Title) == 0 || (len(p.Body) == 0 && len(p.Sections) == 0):
			return fmt.Errorf("config: site.yaml: page %q needs a title and a body or sections", p.ID)
		case pageIDs[p.ID]:
			return fmt.Errorf("config: site.yaml: duplicate page id %q", p.ID)
		case definedIn[p.ID] != "":
			return fmt.Errorf("config: site.yaml: page id %q collides with the service id defined in %s", p.ID, definedIn[p.ID])
		}
		pageIDs[p.ID] = true
		for _, s := range p.Sections {
			if len(s.Title) == 0 || len(s.Body) == 0 {
				return fmt.Errorf("config: site.yaml: page %q: every section needs title and body", p.ID)
			}
		}
	}
	return nil
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

// yaml.v3 phrases its errors for Go programmers ("cannot unmarshal !!str
// into []string", "not found in type main.Site"). The person reading them
// edits a yaml file; yamlErr rewrites the vocabulary into their terms.
var (
	yamlFieldRe = regexp.MustCompile(`field (\S+) not found in type \S+`)
	yamlTypeRe  = regexp.MustCompile("cannot unmarshal (\\S+)( `[^`]*`)? into (\\S+)")
)

func yamlWord(t string) string {
	words := map[string]string{
		"!!str": "a text", "!!seq": "a list", "!!map": "a mapping",
		"!!int": "a number", "!!float": "a number", "!!bool": "a boolean",
		"!!null": "an empty value", "!!timestamp": "a date",
		"string": "one plain text", "[]string": "a list of texts",
		"int": "a number", "bool": "true or false", "*bool": "true or false",
		"map[string]string":   "a per-locale mapping",
		"main.Site":           "the site settings",
		"main.Service":        "a service entry",
		"main.CategoryMeta":   "a category entry",
		"main.imageEntry":     "an image entry",
		"main.SitePage":       "a page entry",
		"main.PageSection":    "a section entry",
		"main.FooterLink":     "a link entry",
		"[]main.FooterLink":   "a list of links",
		"[]main.SitePage":     "a list of pages",
		"[]main.PageSection":  "a list of sections",
		"[]main.ServiceImage": "a list of images",
	}
	if w, ok := words[t]; ok {
		return w
	}
	return strings.TrimPrefix(t, "main.")
}

func yamlErr(err error) string {
	msg := strings.TrimSpace(strings.TrimPrefix(strings.ReplaceAll(err.Error(), "\n", " "), "yaml: unmarshal errors:"))
	msg = yamlFieldRe.ReplaceAllString(msg, `unknown key "$1"`)
	return yamlTypeRe.ReplaceAllStringFunc(msg, func(m string) string {
		p := yamlTypeRe.FindStringSubmatch(m)
		return "found " + yamlWord(p[1]) + p[2] + " where " + yamlWord(p[3]) + " was expected"
	})
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
			return nil, fmt.Errorf("config: %s line %d: %v", file, item.Line, err)
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
			return nil, fmt.Errorf("config: %s line %d: %v", file, item.Line, err)
		}
		switch {
		case s.ID == "":
			return nil, fmt.Errorf("config: %s line %d: service missing id (expected: id: my-tool)", file, item.Line)
		case !idRe.MatchString(s.ID):
			return nil, fmt.Errorf("config: %s line %d: invalid id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. my-tool)", file, item.Line, s.ID)
		case !isURLOrAbs(s.URL):
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
