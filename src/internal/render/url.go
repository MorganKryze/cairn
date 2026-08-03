package render

import (
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
)

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

// AppURL prefixes a root-absolute local path with the base path. Empty values
// and anything already pointing elsewhere pass through untouched, including
// the protocol-relative "//cdn.example.org/x": it starts with a slash but it
// is another origin, and prefixing it produced a path resolving nowhere.
func AppURL(p string) string {
	if !config.IsLocalPath(p) {
		return p
	}
	// One of cairn's own assets goes out under its stamped name, wherever it
	// was built: the touch icon and the manifest's icons come from here and
	// not from a template, and they were the only surfaces left unstamped.
	// Anything the operator supplied passes through untouched.
	return BasePath + stampStatic(p)
}
