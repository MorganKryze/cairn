package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

type Site struct {
	Title   string   `yaml:"title"`
	Tagline LString  `yaml:"tagline"`
	Logo    string   `yaml:"logo"`
	Locales []string `yaml:"locales"`
	Theme   struct {
		Accent string `yaml:"accent"`
	} `yaml:"theme"`
	Footer  []FooterLink       `yaml:"footer"`
	Strings map[string]LString `yaml:"strings"`
}

type Service struct {
	ID       string   `yaml:"id"`
	URL      string   `yaml:"url"`
	Category string   `yaml:"category"`
	Icon     string   `yaml:"icon"`
	Name     LString  `yaml:"name"`
	Desc     LString  `yaml:"desc"`
	Tags     []string `yaml:"tags"`
}

type Category struct {
	ID       string
	Services []Service
}

type Config struct {
	Site       Site
	Categories []Category
}

func (c *Config) DefaultLocale() string { return c.Site.Locales[0] }

var builtinStrings = map[string]map[string]string{
	"en": {
		"nav.skip":           "Skip to content",
		"nav.languages":      "Language",
		"search.label":       "Search",
		"search.placeholder": "Search for a tool…",
		"search.empty":       "No results. Try another word.",
		"cat.other":          "Other",
	},
	"fr": {
		"nav.skip":           "Aller au contenu",
		"nav.languages":      "Langue",
		"search.label":       "Rechercher",
		"search.placeholder": "Chercher un outil…",
		"search.empty":       "Aucun résultat. Essayez un autre mot.",
		"cat.other":          "Autres",
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

func (c *Config) categoryName(id, locale string) string {
	if id == "other" {
		return c.Str(locale, "cat.other")
	}
	name := strings.ReplaceAll(id, "-", " ")
	r := []rune(name)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

var (
	accentRe = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	localeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)
)

func loadConfig(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s (mount your yaml files there): %w", dir, err)
	}

	site := Site{Title: "cairn", Locales: []string{"en"}}
	site.Theme.Accent = "#247b7b"

	var services []Service
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
				return nil, fmt.Errorf("config: %s: %v (expected keys: title, tagline, logo, locales, theme.accent, footer, strings)", name, err)
			}
		case "categories":
			log.Printf("config: ignoring %s (categories.yaml arrives in v0.2)", name)
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

	return &Config{Site: site, Categories: groupCategories(services)}, nil
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
		case !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") && !strings.HasPrefix(s.URL, "/"):
			return nil, fmt.Errorf("config: %s line %d: service %q missing or invalid url (expected: url: https://…)", file, item.Line, s.ID)
		case len(s.Name) == 0:
			return nil, fmt.Errorf("config: %s line %d: service %q missing name (expected: name: My tool  or  name: {fr: …, en: …})", file, item.Line, s.ID)
		}
		out = append(out, s)
	}
	return out, nil
}

// groupCategories derives categories from the services: alphabetical, with
// the implicit "other" bucket last. Services keep the order of their file;
// files merge in name order.
func groupCategories(services []Service) []Category {
	grouped := map[string][]Service{}
	for _, s := range services {
		id := s.Category
		if id == "" {
			id = "other"
		}
		grouped[id] = append(grouped[id], s)
	}
	ids := make([]string, 0, len(grouped))
	for id := range grouped {
		if id != "other" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if _, ok := grouped["other"]; ok {
		ids = append(ids, "other")
	}
	cats := make([]Category, len(ids))
	for i, id := range ids {
		cats[i] = Category{ID: id, Services: grouped[id]}
	}
	return cats
}
