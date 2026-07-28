package config

import (
	"errors"
	"strings"
	"testing"
)

// The YAML vocabulary rewrite: yaml.v3 speaks to Go programmers, the person
// reading the message is editing a text file.
//
// The package-qualified cases matter more than they look. yaml.v3 names our
// own types as "<package>.Type", so the table used to be keyed on "main.Site";
// splitting the code into packages silently turned every one of those friendly
// words back into a Go type name in the operator's face. Matching on the bare
// name fixes it for good, and these cases keep it fixed.
func TestYamlWordSpeaksYAMLNotGo(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"!!str", "a text"},
		{"!!seq", "a list"},
		{"!!int", "a number"},
		{"[]string", "a list of texts"},
		{"map[string]string", "a per-locale mapping"},
		// qualified with whatever package the types happen to live in
		{"config.SitePage", "a page entry"},
		{"config.Service", "a service entry"},
		{"[]config.FooterLink", "a list of links"},
		{"main.Site", "the site settings"},
		{"future.PageSection", "a section entry"},
	} {
		if got := yamlWord(c.in); got != c.want {
			t.Errorf("yamlWord(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// an unknown type still loses its package rather than leaking it
	if got := yamlWord("config.Mystery"); got != "Mystery" {
		t.Errorf("yamlWord on an unknown type = %q, want the package dropped", got)
	}
}

// An unknown key used to be answered with the keys of site.yaml, whatever had
// actually failed: a typo inside a page entry was offered "title, tagline,
// url, logo…", which is not what a page entry accepts. Worse, a service entry
// got no list at all, and the hand-written site list had silently gone stale,
// missing show_version since the day it was added.
//
// The list is now read off the struct, so each of those is answered by the
// shape that failed. This walks every shape a config file can contain.
func TestUnknownKeyAnswersWithTheShapeThatFailed(t *testing.T) {
	for _, c := range []struct {
		name, file, yaml string
		wants            []string
	}{
		{
			name: "site level", file: "site.yaml",
			yaml:  "locales: [en]\ncolour: red\n",
			wants: []string{`unknown key "colour"`, "in the site settings", "tagline", "show_version"},
		},
		{
			name: "an icon entry", file: "site.yaml",
			yaml:  "locales: [en]\nicons:\n  - {src: /assets/b.png, sizes: 512x512, colour: red}\n",
			wants: []string{"in an icon entry", "expected: src, sizes, purpose"},
		},
		{
			name: "a page entry", file: "site.yaml",
			yaml:  "locales: [en]\npages:\n  - {id: legal, title: L, body: B, colour: red}\n",
			wants: []string{"in a page entry", "expected: id, title, body, sections"},
		},
		{
			name: "a section entry", file: "site.yaml",
			yaml:  "locales: [en]\npages:\n  - id: legal\n    title: L\n    sections:\n      - {title: T, body: B, colour: red}\n",
			wants: []string{"in a section entry", "expected: title, body"},
		},
		{
			name: "a link entry", file: "site.yaml",
			yaml:  "locales: [en]\nlinks:\n  - {label: W, url: https://w.example.org, colour: red}\n",
			wants: []string{"in a link entry", "expected: label, url, icon"},
		},
		{
			name: "a service entry", file: "services.yaml",
			yaml:  "- {id: pad, url: https://pad.example.org, name: Pad, colour: red}\n",
			wants: []string{"in a service entry", "expected: id, url, category, icon, name"},
		},
		{
			// The mapping form of an image decodes through a twin type that
			// carries no UnmarshalYAML, and yaml names that twin in its error.
			name: "an image entry", file: "services.yaml",
			yaml:  "- id: pad\n  url: https://pad.example.org\n  name: Pad\n  images:\n    - {src: a.png, caption: C, colour: red}\n",
			wants: []string{"in an image entry", "expected: src, caption"},
		},
		{
			name: "a category entry", file: "categories.yaml",
			yaml:  "- {id: tools, name: Tools, colour: red}\n",
			wants: []string{"in a category entry", "expected: id, name, order"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var err error
			switch c.file {
			case "site.yaml":
				_, err = parseSite(c.file, []byte(c.yaml))
			case "services.yaml":
				_, err = parseServices(c.file, []byte(c.yaml))
			case "categories.yaml":
				_, err = parseCategories(c.file, []byte(c.yaml))
			}
			if err == nil {
				t.Fatal("accepted a file with an unknown key")
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q does not mention %q", err, want)
				}
			}
		})
	}
}

// The point of reading the keys off the struct is that a nested entry is never
// offered the keys of the file it sits in. This is the exact confusion the
// hand-written list produced.
func TestANestedEntryIsNotOfferedTheFilesKeys(t *testing.T) {
	_, err := parseSite("site.yaml", []byte(
		"locales: [en]\nicons:\n  - {src: /assets/b.png, sizes: 512x512, colour: red}\n"))
	if err == nil {
		t.Fatal("accepted an unknown key in an icon entry")
	}
	for _, siteOnly := range []string{"tagline", "locales", "show_version", "theme.accent"} {
		if strings.Contains(err.Error(), siteOnly) {
			t.Errorf("an icon entry was offered %q, a site key: %s", siteOnly, err)
		}
	}
}

// yaml counts lines inside the fragment it was handed, not inside the file.
// A caller that already knows the entry's real line has to drop that, or an
// entry on line 40 is reported as "line 40: line 3:".
func TestEntryErrDropsTheFragmentLine(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"line 3: unknown key \"colour\"", "unknown key \"colour\""},
		{"line 12: found a text where a list was expected", "found a text where a list was expected"},
		{"unknown key \"colour\"", "unknown key \"colour\""}, // nothing to drop
		{"outline 3: kept", "outline 3: kept"},               // only a real prefix goes
	} {
		if got := entryErr(errors.New(c.in)); got != c.want {
			t.Errorf("entryErr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
