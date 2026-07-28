package config

import (
	"regexp"
	"strings"
)

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
		"map[string]string": "a per-locale mapping",
		"Site":              "the site settings",
		"Service":           "a service entry",
		"CategoryMeta":      "a category entry",
		"SitePage":          "a page entry",
		"PageSection":       "a section entry",
		"FooterLink":        "a link entry",
		"ServiceImage":      "an image entry",
		"[]FooterLink":      "a list of links",
		"[]SitePage":        "a list of pages",
		"[]PageSection":     "a list of sections",
		"[]ServiceImage":    "a list of images",
	}
	if w, ok := words[t]; ok {
		return w
	}
	// yaml.v3 qualifies our own types with the package they live in, which is
	// an implementation detail the operator never needs and which silently
	// changed the day the packages were split. Match on the bare name.
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

func yamlErr(err error) string {
	msg := strings.TrimSpace(strings.TrimPrefix(strings.ReplaceAll(err.Error(), "\n", " "), "yaml: unmarshal errors:"))
	msg = yamlFieldRe.ReplaceAllString(msg, `unknown key "$1"`)
	return yamlTypeRe.ReplaceAllStringFunc(msg, func(m string) string {
		p := yamlTypeRe.FindStringSubmatch(m)
		return "found " + yamlWord(p[1]) + p[2] + " where " + yamlWord(p[3]) + " was expected"
	})
}
