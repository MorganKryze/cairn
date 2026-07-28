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
