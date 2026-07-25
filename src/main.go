package main

import (
	"encoding/json"
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
	"sync"
	"sync/atomic"
	"time"
)

var current atomic.Pointer[Model]

// reloadMu serializes the two writers of current (config watcher and status
// poller) so neither clobbers the other's half of the model.
var reloadMu sync.Mutex

// version is stamped by the build: -ldflags "-X main.version=…"
var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	cfgDir := flag.String("config", "/config", "config directory")
	assetsDir := flag.String("assets", "/assets", "optional directory served at /assets/")
	check := flag.Bool("healthcheck", false, "probe the running server and exit (for container healthchecks)")
	validate := flag.Bool("check", false, "validate the config directory, print warnings, and exit (0 ok, 1 error)")
	emit := flag.Bool("emit-gatus", false, "print a Gatus endpoints config derived from the services and exit")
	emitIcons := flag.Bool("emit-icons", false, "print a shell script that downloads your icon slugs for self-hosting and exit")
	initCfg := flag.Bool("init", false, "print a commented starter services.yaml and exit")
	ver := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	assetsPath = *assetsDir

	if *ver {
		fmt.Println("cairn", version)
		return
	}
	if *initCfg {
		fmt.Print(starterServices)
		return
	}
	if *check {
		os.Exit(probe(*addr))
	}
	if *validate {
		os.Exit(runCheck(*cfgDir))
	}

	cfg, err := loadConfig(*cfgDir)
	if *emit || *emitIcons {
		if err != nil {
			log.Fatal(err)
		}
		var out []byte
		if *emit {
			out, err = emitGatus(cfg)
		} else {
			out = emitIconsScript(cfg)
		}
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout.Write(out)
		return
	}
	if err != nil {
		// A dead container helps nobody: serve the getting-started page and
		// let the watcher swap the real config in the moment it is valid.
		log.Print(err)
		log.Printf("no valid config yet: serving the getting-started page, watching %s", *cfgDir)
		current.Store(starterModel())
	} else {
		m, merr := buildModel(cfg, nil)
		if merr != nil {
			log.Fatal(merr)
		}
		current.Store(m)
	}
	go watch(*cfgDir)
	go pollStatus()

	mux := http.NewServeMux()
	static, _ := fs.Sub(embedded, "assets")
	mux.Handle("GET /static/", cacheControl(http.StripPrefix("/static/", http.FileServerFS(static))))
	// Mounted unconditionally: a dir that appears after boot just works.
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", noListing(http.FileServer(http.Dir(*assetsDir)))))
	// Service preview images live next to the yaml, in <config>/media/.
	mux.Handle("GET /media/", http.StripPrefix("/media/", noListing(http.FileServer(http.Dir(filepath.Join(*cfgDir, "media"))))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	mux.HandleFunc("GET /custom.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(*cfgDir, "custom.css"))
	})
	mux.HandleFunc("GET /manifest.webmanifest", manifest)
	mux.HandleFunc("GET /robots.txt", robots)
	mux.HandleFunc("GET /sitemap.xml", sitemap)
	mux.HandleFunc("GET /{$}", root)
	// Catch-all rather than /{locale}/{$}: a wildcard pattern would conflict
	// with the /static/ and /assets/ subtrees in the 1.22 mux.
	mux.HandleFunc("GET /", home)

	cfg = current.Load().Cfg
	log.Printf("cairn %s: %d services, locales %v, listening on %s", version, countServices(cfg), cfg.Site.Locales, *addr)
	// ReadHeaderTimeout caps slow-header (Slowloris) clients; the request body
	// stays untimed since cairn only ever reads tiny GETs.
	srv := &http.Server{Addr: *addr, Handler: secureHeaders(mux), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
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
	// Local icons live outside the config dir but change the pages too.
	iconsDir := filepath.Join(assetsPath, "icons")
	last := fingerprint(dir) + fingerprint(iconsDir)
	for range time.Tick(2 * time.Second) {
		fp := fingerprint(dir) + fingerprint(iconsDir)
		if fp == last {
			continue
		}
		last = fp
		cfg, err := loadConfig(dir)
		if err != nil {
			log.Printf("reload failed, keeping previous config: %v", err)
			continue
		}
		reloadMu.Lock()
		m, err := buildModel(cfg, current.Load().Statuses)
		if err == nil {
			current.Store(m)
		}
		reloadMu.Unlock()
		if err != nil {
			log.Printf("reload failed, keeping previous config: %v", err)
			continue
		}
		log.Printf("config reloaded: %d services, locales %v", countServices(cfg), cfg.Site.Locales)
	}
}

// pollStatus feeds the status dots from the Gatus API, server-side only. On
// any fetch problem the dots disappear rather than go stale.
func pollStatus() {
	if cfg := current.Load().Cfg; cfg.Site.Status.Gatus != "" {
		log.Printf("status: polling gatus at %s every %s", cfg.Site.Status.Gatus, cfg.StatusInterval())
	}
	var lastErr, lastMissing string
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
				if msg := unmonitored(m.Cfg, st); msg != lastMissing {
					if msg != "" {
						log.Printf("status: %s", msg)
					}
					lastMissing = msg
				}
			}
			reloadMu.Lock()
			// Re-read under the lock so a config reload that landed mid-poll
			// is merged with, not overwritten by, the fresh statuses.
			cur := current.Load()
			if !maps.Equal(st, cur.Statuses) {
				if next, err := buildModel(cur.Cfg, st); err == nil {
					current.Store(next)
				} else {
					log.Printf("status: render failed, keeping previous pages: %v", err)
				}
			}
			reloadMu.Unlock()
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
		http.SetCookie(w, &http.Cookie{Name: "locale", Value: locale, Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode, Secure: secureRequest(r)})
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

// manifest makes "add to home screen" give the site its own name and icon;
// deliberately no service worker and no offline mode.
func manifest(w http.ResponseWriter, r *http.Request) {
	cfg := current.Load().Cfg
	name := cfg.Site.Title.Get(negotiate(r, cfg.Site.Locales), cfg.DefaultLocale())
	icon := touchIcon(cfg.Site.Favicon)
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"name":             name,
		"short_name":       name,
		"start_url":        "/",
		"display":          "minimal-ui",
		"background_color": "#eef0ea",
		"theme_color":      cfg.Site.Theme.Accent,
		"icons":            []map[string]string{{"src": icon, "sizes": "180x180", "type": "image/png"}},
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

func noindex() bool { return current.Load().Cfg.Noindex() }

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

// noListing serves files but never directory indexes.
func noListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// After StripPrefix the folder root is the empty path.
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func cacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		h.ServeHTTP(w, r)
	})
}
