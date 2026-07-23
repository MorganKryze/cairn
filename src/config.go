package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
		return err
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
}

// SitePage is a page cairn serves itself (legal notice, privacy…), linked
// automatically in the footer after the manual entries.
type SitePage struct {
	ID    string  `yaml:"id"`
	Title LString `yaml:"title"`
	Body  LString `yaml:"body"`
}

type Site struct {
	Title   string   `yaml:"title"`
	Tagline LString  `yaml:"tagline"`
	Logo    string   `yaml:"logo"`
	Locales []string `yaml:"locales"`
	Theme   struct {
		Accent string `yaml:"accent"`
	} `yaml:"theme"`
	Links   []FooterLink       `yaml:"links"`
	Footer  []FooterLink       `yaml:"footer"`
	Pages   []SitePage         `yaml:"pages"`
	Strings map[string]LString `yaml:"strings"`
	Status  struct {
		Gatus    string `yaml:"gatus"`
		Page     string `yaml:"page"`
		Interval string `yaml:"interval"`
	} `yaml:"status"`
}

type Service struct {
	ID       string   `yaml:"id"`
	URL      string   `yaml:"url"`
	Category string   `yaml:"category"`
	Icon     string   `yaml:"icon"`
	Name     LString  `yaml:"name"`
	Desc     LString  `yaml:"desc"`
	Details  LString  `yaml:"details"`
	Tags     []string `yaml:"tags"`
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
}

func (c *Config) DefaultLocale() string { return c.Site.Locales[0] }

func (c *Config) StatusInterval() time.Duration {
	if d, err := time.ParseDuration(c.Site.Status.Interval); err == nil && d >= 5*time.Second {
		return d
	}
	return 60 * time.Second
}

var builtinStrings = map[string]map[string]string{
	"en": {
		"nav.skip":           "Skip to content",
		"nav.languages":      "Language",
		"nav.toc":            "Categories",
		"nav.links":          "Links",
		"search.label":       "Search",
		"search.placeholder": "Search for a tool…",
		"search.empty":       "No results. Try another word.",
		"cat.other":          "Other",
		"card.more":          "Learn more",
		"detail.open":        "Open the tool",
		"detail.back":        "Back",
		"status.up":          "Online",
		"status.down":        "Offline",
		"status.unknown":     "Unknown",
		"status.link":        "view status",
	},
	"fr": {
		"nav.skip":           "Aller au contenu",
		"nav.languages":      "Langue",
		"nav.toc":            "Catégories",
		"nav.links":          "Liens",
		"search.label":       "Rechercher",
		"search.placeholder": "Chercher un outil…",
		"search.empty":       "Aucun résultat. Essayez un autre mot.",
		"cat.other":          "Autres",
		"card.more":          "En savoir plus",
		"detail.open":        "Ouvrir l’outil",
		"detail.back":        "Retour",
		"status.up":          "En ligne",
		"status.down":        "Hors ligne",
		"status.unknown":     "Inconnu",
		"status.link":        "voir le statut",
	},
}

// Str resolves a UI string: site.yaml override (only for the locales it
// defines), then the locale's built-in set, then English, then the key.
func (c *Config) Str(locale, key string) string {
	if ls, ok := c.Site.Strings[key]; ok {
		for _, k := range []string{locale, ""} {
			if s := ls[k]; s != "" {
				return s
			}
		}
	}
	if s := builtinStrings[locale][key]; s != "" {
		return s
	}
	if s := builtinStrings["en"][key]; s != "" {
		return s
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

func loadConfig(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s (mount your yaml files there): %w", dir, err)
	}

	site := Site{Title: "cairn", Locales: []string{"en"}}
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
				return nil, fmt.Errorf("config: %s: %v (expected keys: title, tagline, logo, locales, theme.accent, links, footer, pages, strings, status)", name, err)
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
	if len(site.Locales) == 0 {
		site.Locales = []string{"en"}
	}
	for _, l := range site.Locales {
		if !localeRe.MatchString(l) {
			return nil, fmt.Errorf("config: site.yaml: invalid locale %q (expected codes like fr, en, pt-BR)", l)
		}
	}
	if !accentRe.MatchString(site.Theme.Accent) {
		return nil, fmt.Errorf("config: site.yaml: theme.accent %q is not a hex color (expected e.g. \"#247b7b\")", site.Theme.Accent)
	}
	if g := site.Status.Gatus; g != "" && !strings.HasPrefix(g, "http://") && !strings.HasPrefix(g, "https://") {
		return nil, fmt.Errorf("config: site.yaml: status.gatus %q is not a URL (expected e.g. https://status.example.org)", g)
	}
	if p := site.Status.Page; p != "" && !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") {
		return nil, fmt.Errorf("config: site.yaml: status.page %q is not a URL (expected e.g. https://status.example.org)", p)
	}
	if iv := site.Status.Interval; iv != "" {
		if d, err := time.ParseDuration(iv); err != nil || d < 5*time.Second {
			return nil, fmt.Errorf("config: site.yaml: status.interval %q is not a duration of at least 5s (expected e.g. 60s)", iv)
		}
	}
	for _, l := range site.Links {
		if len(l.Label) == 0 || l.URL == "" {
			return nil, fmt.Errorf("config: site.yaml: every links entry needs label and url (expected: - {label: Wiki, url: https://…})")
		}
	}
	pageIDs := map[string]bool{}
	for _, p := range site.Pages {
		switch {
		case !idRe.MatchString(p.ID):
			return nil, fmt.Errorf("config: site.yaml: invalid page id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. legal)", p.ID)
		case len(p.Title) == 0 || len(p.Body) == 0:
			return nil, fmt.Errorf("config: site.yaml: page %q needs title and body", p.ID)
		case pageIDs[p.ID]:
			return nil, fmt.Errorf("config: site.yaml: duplicate page id %q", p.ID)
		case definedIn[p.ID] != "":
			return nil, fmt.Errorf("config: site.yaml: page id %q collides with the service id defined in %s", p.ID, definedIn[p.ID])
		}
		pageIDs[p.ID] = true
	}

	cfg := &Config{Site: site, Categories: groupCategories(services, metas)}
	if st, err := os.Stat(filepath.Join(dir, "custom.css")); err == nil && !st.IsDir() {
		cfg.CustomCSS = true
	}
	return cfg, nil
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
		if err := item.Decode(&m); err != nil {
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
		if err := item.Decode(&s); err != nil {
			return nil, fmt.Errorf("config: %s line %d: %v", file, item.Line, err)
		}
		switch {
		case s.ID == "":
			return nil, fmt.Errorf("config: %s line %d: service missing id (expected: id: my-tool)", file, item.Line)
		case !idRe.MatchString(s.ID):
			return nil, fmt.Errorf("config: %s line %d: invalid id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. my-tool)", file, item.Line, s.ID)
		case !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") && !strings.HasPrefix(s.URL, "/"):
			return nil, fmt.Errorf("config: %s line %d: service %q missing or invalid url (expected: url: https://…)", file, item.Line, s.ID)
		case len(s.Name) == 0:
			return nil, fmt.Errorf("config: %s line %d: service %q missing name (expected: name: My tool  or  name: {fr: …, en: …})", file, item.Line, s.ID)
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
