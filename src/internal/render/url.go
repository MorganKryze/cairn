package render

import "strings"

// BasePath mirrors the -base-path flag: "" at the domain root, "/cairn" when
// the site is mounted under a sub-path. Every URL cairn generates carries it,
// and the router strips it back off, so the proxy in front needs no rewriting.
// It is fixed for the life of the process on purpose: pages are pre-rendered
// once per config, so the prefix cannot be a per-request header.
var BasePath = ""

// NormalizeBase turns what an operator may type ("cairn", "/cairn/", "/")
// into the one shape the rest of the code expects: "" or "/cairn".
func NormalizeBase(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// AppURL prefixes a root-absolute local path with the base path. Empty
// values and absolute URLs (icons on a CDN, operator links) pass through.
func AppURL(p string) string {
	if !strings.HasPrefix(p, "/") {
		return p
	}
	return BasePath + p
}
