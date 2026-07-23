package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var current atomic.Pointer[Model]

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	cfgDir := flag.String("config", "/config", "config directory")
	assetsDir := flag.String("assets", "/assets", "optional directory served at /assets/")
	check := flag.Bool("healthcheck", false, "probe the running server and exit (for container healthchecks)")
	emit := flag.Bool("emit-gatus", false, "print a Gatus endpoints config derived from the services and exit")
	flag.Parse()

	if *check {
		os.Exit(probe(*addr))
	}

	cfg, err := loadConfig(*cfgDir)
	if err != nil {
		log.Fatal(err)
	}
	if *emit {
		out, err := emitGatus(cfg)
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout.Write(out)
		return
	}
	m, err := buildModel(cfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	current.Store(m)
	go watch(*cfgDir)
	go pollStatus()

	mux := http.NewServeMux()
	static, _ := fs.Sub(embedded, "assets")
	mux.Handle("GET /static/", cacheControl(http.StripPrefix("/static/", http.FileServerFS(static))))
	if st, err := os.Stat(*assetsDir); err == nil && st.IsDir() {
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assetsDir))))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	mux.HandleFunc("GET /custom.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(*cfgDir, "custom.css"))
	})
	mux.HandleFunc("GET /robots.txt", robots)
	mux.HandleFunc("GET /sitemap.xml", sitemap)
	mux.HandleFunc("GET /{$}", root)
	// Catch-all rather than /{locale}/{$}: a wildcard pattern would conflict
	// with the /static/ and /assets/ subtrees in the 1.22 mux.
	mux.HandleFunc("GET /", home)

	log.Printf("cairn: %d services, locales %v, listening on %s", countServices(cfg), cfg.Site.Locales, *addr)
	log.Fatal(http.ListenAndServe(*addr, secureHeaders(mux)))
}

// secureHeaders sets the response headers a security scan would ask for.
// The CSP whitelists exactly what the pages use; see buildCSP.
func secureHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		hd.Set("Content-Security-Policy", current.Load().CSP)
		h.ServeHTTP(w, r)
	})
}

func countServices(cfg *Config) int {
	n := 0
	for _, c := range cfg.Categories {
		n += len(c.Services)
	}
	return n
}

func probe(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// watch polls instead of using inotify: polling survives bind mounts and
// configmap symlink swaps that file watchers routinely miss.
func watch(dir string) {
	last := fingerprint(dir)
	for range time.Tick(2 * time.Second) {
		fp := fingerprint(dir)
		if fp == last {
			continue
		}
		last = fp
		cfg, err := loadConfig(dir)
		if err != nil {
			log.Printf("reload failed, keeping previous config: %v", err)
			continue
		}
		m, err := buildModel(cfg, current.Load().Statuses)
		if err != nil {
			log.Printf("reload failed, keeping previous config: %v", err)
			continue
		}
		current.Store(m)
		log.Printf("config reloaded: %d services, locales %v", countServices(cfg), cfg.Site.Locales)
	}
}

// pollStatus feeds the status dots from the Gatus API, server-side only. On
// any fetch problem the dots disappear rather than go stale.
func pollStatus() {
	var lastErr string
	for {
		m := current.Load()
		if url := m.Cfg.Site.Status.Gatus; url != "" {
			st, err := fetchStatuses(url)
			if err != nil {
				st = nil
				if err.Error() != lastErr {
					log.Printf("status: %v (dots hidden until gatus answers)", err)
					lastErr = err.Error()
				}
			} else {
				lastErr = ""
			}
			if !maps.Equal(st, m.Statuses) {
				if next, err := buildModel(m.Cfg, st); err == nil {
					current.Store(next)
				} else {
					log.Printf("status: render failed, keeping previous pages: %v", err)
				}
			}
		}
		time.Sleep(m.Cfg.StatusInterval())
	}
}

func fingerprint(dir string) string {
	var b strings.Builder
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			fmt.Fprintf(&b, "%s|%d|%d;", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}

func root(w http.ResponseWriter, r *http.Request) {
	m := current.Load()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/"+negotiate(r, m.Cfg.Site.Locales)+"/", http.StatusFound)
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
	m := current.Load()
	loc := strings.Trim(r.URL.Path, "/")
	page, ok := m.Pages[loc]
	if !ok {
		if strings.Contains(loc, "/") {
			http.NotFound(w, r)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
		return
	}
	if r.URL.Path != "/"+loc+"/" {
		http.Redirect(w, r, "/"+loc+"/", http.StatusMovedPermanently)
		return
	}
	locale, _, _ := strings.Cut(loc, "/")
	// The cookie is an explicit choice only: the switcher links carry
	// ?choose. A negotiated visit leaves no trace, so a visitor whose
	// browser language changes is followed until they pick one themselves.
	if r.URL.Query().Has("choose") {
		http.SetCookie(w, &http.Cookie{Name: "locale", Value: locale, Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode})
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, "/"+loc+"/", http.StatusFound)
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

// siteBase prefers the configured public URL; without one it falls back to
// what the request headers suggest.
func siteBase(r *http.Request) string {
	if u := current.Load().Cfg.Site.URL; u != "" {
		return u
	}
	return baseURL(r)
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

func robots(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", siteBase(r))
}

func sitemap(w http.ResponseWriter, r *http.Request) {
	m := current.Load()
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

func cacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		h.ServeHTTP(w, r)
	})
}
