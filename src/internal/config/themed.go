package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ThemedRef is an image that may be drawn differently in the two themes: a
// logo, a favicon, a service icon.
//
// Most artwork needs one file. A monochrome mark needs two, and the reason is
// measurable rather than a matter of taste: on cairn's tile, a black mark
// reads 1.40:1 against the dark theme and a white one 1.24:1 against the
// light, where the rules ask 3:1. One of the two is always close to invisible.
//
// A plain string is the field every config written so far carries, and it goes
// on meaning exactly what it meant: one image, both themes. The pair names the
// theme each file appears in, rather than the ink it is drawn with. That is
// deliberate: the icon collections suffix the ink instead, so
// dashboard-icons' github-light.svg is the pale one for a dark background, and
// a key named dark: holding a file named -light is confusing exactly once,
// where a key that named the ink would be confusing every time.
type ThemedRef struct {
	Light string
	Dark  string
}

func (t *ThemedRef) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*t = ThemedRef{Light: n.Value}
		return nil
	}
	var m map[string]string
	if err := n.Decode(&m); err != nil {
		return fmt.Errorf("line %d: expected one path or slug, or the pair {light: …, dark: …}", n.Line)
	}
	for k := range m {
		if k != "light" && k != "dark" {
			return fmt.Errorf("line %d: %q is not a theme; the two keys are light and dark, naming the theme each image appears in", n.Line, k)
		}
	}
	// Half a pair paints nothing in one theme and leaves no trace of why, so
	// it stops here rather than at the first visitor who switches.
	if m["light"] == "" || m["dark"] == "" {
		return fmt.Errorf("line %d: needs both light and dark; write one plain value instead if the same image serves both themes", n.Line)
	}
	*t = ThemedRef{Light: m["light"], Dark: m["dark"]}
	return nil
}

// Themed reports whether the two themes genuinely differ, which is what
// decides between plain markup and a rule in the page's stylesheet.
func (t ThemedRef) Themed() bool { return t.Dark != "" && t.Dark != t.Light }

// ThemedField is one half of a themed image, carrying the name to blame. A
// refusal has to point at the line to fix, and "logo" is not that line when
// only the dark one is wrong.
type ThemedField struct{ Key, Val string }

// Fields expands a themed image into the values to check, naming the variant
// only when there are two to tell apart, so a site with one logo still reads
// "logo" in every message it has ever produced.
func (t ThemedRef) Fields(key string) []ThemedField {
	if !t.Themed() {
		return []ThemedField{{key, t.Light}}
	}
	return []ThemedField{{key + ".light", t.Light}, {key + ".dark", t.Dark}}
}

// Refs lists the values actually written, for the checks and the emitters that
// have to walk every image an operator named.
func (t ThemedRef) Refs() []string {
	switch {
	case t.Light == "":
		return nil
	case t.Themed():
		return []string{t.Light, t.Dark}
	default:
		return []string{t.Light}
	}
}
