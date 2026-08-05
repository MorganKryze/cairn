package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// yaml.v3 phrases its errors for Go programmers ("cannot unmarshal !!str
// into []string", "not found in type main.Site"). The person reading them
// edits a yaml file; yamlErr rewrites the vocabulary into their terms.
var (
	yamlFieldRe = regexp.MustCompile(`field (\S+) not found in type (\S+)`)
	yamlTypeRe  = regexp.MustCompile("cannot unmarshal (\\S+)( `[^`]*`)? into (\\S+)")
)

func yamlWord(t string) string {
	words := map[string]string{
		"!!str": "a text", "!!seq": "a list", "!!map": "a mapping",
		"!!int": "a number", "!!float": "a number", "!!bool": "a boolean",
		"!!null": "an empty value", "!!timestamp": "a date",
		"string": "one plain text", "[]string": "a list of texts",
		"int": "a number", "bool": "true or false", "*bool": "true or false",
		"map[string]string": "a per-locale mapping",
		"Site":              "the site settings",
		"Service":           "a service entry",
		"CategoryMeta":      "a category entry",
		"SitePage":          "a page entry",
		"PageSection":       "a section entry",
		"FooterLink":        "a link entry",
		"ServiceImage":      "an image entry",
		"imageEntry":        "an image entry", // the decoding twin of the above
		"SiteIcon":          "an icon entry",
		"[]FooterLink":      "a list of links",
		"[]SitePage":        "a list of pages",
		"[]PageSection":     "a list of sections",
		"[]ServiceImage":    "a list of images",
	}
	if w, ok := words[t]; ok {
		return w
	}
	// yaml.v3 qualifies our own types with the package they live in, which the
	// operator never needs and which changed when the packages were split.
	// Match on the bare name.
	if bare := dropPackage(t); bare != t {
		if w, ok := words[bare]; ok {
			return w
		}
		return bare
	}
	return t
}

// dropPackage turns "config.SitePage" into "SitePage" and "[]config.SitePage"
// into "[]SitePage", leaving anything unqualified alone.
func dropPackage(t string) string {
	prefix := ""
	if rest, ok := strings.CutPrefix(t, "[]"); ok {
		prefix, t = "[]", rest
	}
	if _, name, ok := strings.Cut(t, "."); ok {
		return prefix + name
	}
	return prefix + t
}

// yamlKeys lists the keys a struct accepts, in declaration order. An inline
// struct contributes its own keys dotted onto its name, because "theme.accent"
// is how the operator writes it.
func yamlKeys(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		if f.Type.Kind() == reflect.Struct && f.Type.Name() == "" {
			for _, sub := range yamlKeys(f.Type) {
				out = append(out, name+"."+sub)
			}
			continue
		}
		out = append(out, name)
	}
	return out
}

// yamlShapes maps every named struct reachable from a config file to itself,
// so an "unknown key" error can offer the keys of the entry that actually
// failed. Reflection rather than a written list: the hand-kept one had
// drifted, and a nested entry could only ever be offered the top-level keys.
var yamlShapes = func() map[string]reflect.Type {
	shapes := map[string]reflect.Type{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Slice || t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || t.Name() == "" || shapes[t.Name()] != nil {
			return
		}
		shapes[t.Name()] = t
		for i := range t.NumField() {
			walk(t.Field(i).Type)
		}
	}
	for _, root := range []any{Site{}, Service{}, CategoryMeta{}, imageEntry{}} {
		walk(reflect.TypeOf(root))
	}
	return shapes
}()

// innerLineRe matches the line number yaml counts inside a re-marshalled
// fragment. It is meaningless outside that fragment: an entry at line 40 of
// the file would be reported as line 3 of a snippet nobody can see.
var innerLineRe = regexp.MustCompile(`^line \d+: `)

// entryErr is yamlErr for a caller that already names the entry's real line.
func entryErr(err error) string {
	return innerLineRe.ReplaceAllString(err.Error(), "")
}

func unknownKey(field, typ string) string {
	out := fmt.Sprintf("unknown key %q", field)
	shape, ok := yamlShapes[dropPackage(typ)]
	if !ok {
		return out
	}
	if word := yamlWord(typ); word != typ {
		out += " in " + word
	}
	return out + " (expected: " + strings.Join(yamlKeys(shape), ", ") + ")"
}

func yamlErr(err error) string {
	msg := strings.TrimSpace(strings.TrimPrefix(strings.ReplaceAll(err.Error(), "\n", " "), "yaml: unmarshal errors:"))
	msg = yamlFieldRe.ReplaceAllStringFunc(msg, func(m string) string {
		p := yamlFieldRe.FindStringSubmatch(m)
		return unknownKey(p[1], p[2])
	})
	return yamlTypeRe.ReplaceAllStringFunc(msg, func(m string) string {
		p := yamlTypeRe.FindStringSubmatch(m)
		return "found " + yamlWord(p[1]) + p[2] + " where " + yamlWord(p[3]) + " was expected"
	})
}
