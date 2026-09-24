package export

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/server"
)

// rules writes what a static host needs to answer the way the server would:
// _headers and _redirects for Cloudflare Pages and Netlify, .htaccess for
// Apache, which is what OVH and most shared hosting run. Each host ignores the
// other's files.
//
// The paths in them carry the base path, because a host reads them against
// the domain root: under -base-path the site sits in a subdirectory, and on
// Cloudflare Pages or Netlify these two files move up to the root with it.
func (e *exporter) rules(cfg *config.Config) error {
	req := httptest.NewRequest(http.MethodGet, render.BasePath+"/", nil)
	req.Header.Set("X-Forwarded-Proto", e.site.Scheme)
	hard := server.Hardening(req)
	names := make([]string, 0, len(hard))
	for k := range hard {
		names = append(names, k)
	}
	sort.Strings(names)
	sort.Strings(e.immutable)

	var h strings.Builder
	fmt.Fprintf(&h, "%s/*\n", render.BasePath)
	for _, k := range names {
		fmt.Fprintf(&h, "  %s: %s\n", k, hard.Get(k))
	}
	for _, f := range e.immutable {
		fmt.Fprintf(&h, "\n%s/static/%s\n  Cache-Control: %s\n", render.BasePath, f, e.forever)
	}
	if err := e.add("_headers", []byte(h.String())); err != nil {
		return err
	}

	if len(e.redirects) > 0 {
		var r strings.Builder
		for _, rd := range e.redirects {
			fmt.Fprintf(&r, "%s %s 302\n", rd.from, rd.to)
		}
		if err := e.add("_redirects", []byte(r.String())); err != nil {
			return err
		}
	}

	var a strings.Builder
	a.WriteString("Options -Indexes\n")
	a.WriteString("AddDefaultCharset utf-8\n")
	a.WriteString("AddType application/manifest+json .webmanifest\n")
	a.WriteString("AddType font/woff2 .woff2\n\n")
	fmt.Fprintf(&a, "ErrorDocument 404 %s/404.html\n", render.BasePath)
	for _, loc := range cfg.Site.Locales {
		fmt.Fprintf(&a, "<If \"%%{REQUEST_URI} =~ m#^%s/#\">\n  ErrorDocument 404 %s/%s/404.html\n</If>\n",
			regexp.QuoteMeta(render.BasePath+"/"+loc), render.BasePath, loc)
	}
	for _, rd := range e.redirects {
		fmt.Fprintf(&a, "Redirect 302 %s %s\n", rd.from, rd.to)
	}
	a.WriteString("\n<FilesMatch \"^(_headers|_redirects|\\.htaccess)$\">\n  Require all denied\n</FilesMatch>\n")
	a.WriteString("\n<IfModule mod_headers.c>\n")
	for _, k := range names {
		fmt.Fprintf(&a, "  Header always set %s %q\n", k, hard.Get(k))
	}
	if len(e.immutable) > 0 {
		quoted := make([]string, len(e.immutable))
		for i, f := range e.immutable {
			quoted[i] = regexp.QuoteMeta(f)
		}
		fmt.Fprintf(&a, "  <FilesMatch \"^(%s)$\">\n    Header set Cache-Control %q\n  </FilesMatch>\n", strings.Join(quoted, "|"), e.forever)
	}
	a.WriteString("</IfModule>\n")
	return e.add(".htaccess", []byte(a.String()))
}
