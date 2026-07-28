package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/render"
)

func root(w http.ResponseWriter, r *http.Request) {
	m := Current()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, render.BasePath+"/"+negotiate(r, m.Cfg.Site.Locales)+"/", http.StatusFound)
}

func negotiate(r *http.Request, locales []string) string {
	if c, err := r.Cookie("locale"); err == nil && slices.Contains(locales, c.Value) {
		return c.Value
	}
	type cand struct {
		tag string
		q   float64
	}
	var cands []cand
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if tag == "" {
			continue
		}
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				q = f
			}
		}
		cands = append(cands, cand{strings.TrimSpace(tag), q})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].q > cands[j].q })
	base := func(t string) string { b, _, _ := strings.Cut(t, "-"); return strings.ToLower(b) }
	for _, c := range cands {
		for _, l := range locales {
			if strings.EqualFold(c.tag, l) || base(c.tag) == base(l) {
				return l
			}
		}
	}
	return locales[0]
}

func home(w http.ResponseWriter, r *http.Request) {
	m := Current()
	loc := strings.Trim(r.URL.Path, "/")
	page, ok := m.Pages[loc]
	if !ok {
		if strings.Contains(loc, "/") {
			http.NotFound(w, r)
		} else {
			http.Redirect(w, r, render.BasePath+"/", http.StatusFound)
		}
		return
	}
	if r.URL.Path != "/"+loc+"/" {
		http.Redirect(w, r, render.BasePath+"/"+loc+"/", http.StatusMovedPermanently)
		return
	}
	locale, _, _ := strings.Cut(loc, "/")
	// The cookie is an explicit choice only: the switcher links carry
	// ?choose. A negotiated visit leaves no trace, so a visitor whose
	// browser language changes is followed until they pick one themselves.
	if r.URL.Query().Has("choose") {
		http.SetCookie(w, &http.Cookie{Name: "locale", Value: locale, Path: render.BasePath + "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode, Secure: secureRequest(r)})
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, render.BasePath+"/"+loc+"/", http.StatusFound)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Language", locale)
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", page.ETag)
	if r.Header.Get("If-None-Match") == page.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Write(page.HTML)
}

// faviconICO answers the well-known path. No browser asks for it, since the
// head declares the icons; it is here for the feed readers, link previewers
// and bookmark tools that skip the html and fetch /favicon.ico directly.
//
// An operator who set their own favicon gets pointed at it. Serving cairn's
// stones to a link previewer would put our mark on their site's card.
func faviconICO(static fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if own := Current().Cfg.Site.Favicon; own != "" {
			http.Redirect(w, r, render.AppURL(own), http.StatusFound)
			return
		}
		http.ServeFileFS(w, r, static, "favicon.ico")
	}
}

// manifest makes "add to home screen" give the site its own name and icon;
// deliberately no service worker and no offline mode.
func manifest(w http.ResponseWriter, r *http.Request) {
	cfg := Current().Cfg
	name := cfg.Site.Title.Get(negotiate(r, cfg.Site.Locales), cfg.DefaultLocale())
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"name":             name,
		"short_name":       name,
		"start_url":        render.BasePath + "/",
		"display":          "minimal-ui",
		"background_color": "#eef0ea",
		"theme_color":      cfg.Site.Theme.Accent,
		"icons":            render.AppIcons(cfg.Site.Favicon),
	}); err != nil {
		log.Printf("manifest: %v", err)
	}
}

// secureRequest reports whether the visitor reached us over https, directly
// or behind a proxy. The cookies carry Secure exactly then, so they keep
// working on the plain-http LAN deployments cairn is also made for.
func secureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// siteBase prefers the configured public URL; without one it falls back to
// what the request headers suggest.
func siteBase(r *http.Request) string {
	if u := Current().Cfg.Site.URL; u != "" {
		return u + render.BasePath
	}
	return baseURL(r) + render.BasePath
}

func baseURL(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}

func noindex() bool { return Current().Cfg.Noindex() }

func robots(w http.ResponseWriter, r *http.Request) {
	if noindex() {
		io.WriteString(w, "User-agent: *\nDisallow: /\n")
		return
	}
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", siteBase(r))
}

func sitemap(w http.ResponseWriter, r *http.Request) {
	if noindex() {
		http.NotFound(w, r)
		return
	}
	m := Current()
	keys := make([]string, 0, len(m.Pages))
	for k := range m.Pages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	io.WriteString(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, k := range keys {
		fmt.Fprintf(w, "  <url><loc>%s/%s/</loc></url>\n", siteBase(r), k)
	}
	io.WriteString(w, "</urlset>\n")
}
