package render

import (
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// BasePath mirrors the -base-path flag: "" at the domain root, "/cairn" when the
// site is mounted under a sub-path. Every URL cairn generates carries it and the
// router strips it back off, so the proxy in front needs no rewriting. Pages are
// pre-rendered once per config, so it is fixed for the life of the process and
// cannot become a per-request header.
var BasePath = ""

func NormalizeBase(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// AppURL prefixes a root-absolute local path with the base path. Anything
// pointing elsewhere passes through untouched, the protocol-relative
// "//cdn.example.org/x" included: it starts with a slash but names another
// origin, and prefixing it produces a path that resolves nowhere.
func AppURL(p string) string {
	if !config.IsLocalPath(p) {
		return p
	}
	return BasePath + stampStatic(p)
}
