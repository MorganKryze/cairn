// Package server is everything HTTP: the live model, the routes, the
// middleware and the two background loops that keep the model fresh.
package server

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MorganKryze/cairn/src/internal/render"
)

// current is the whole served site: an immutable model swapped atomically on
// config reload or status change, so a request never sees a half-built page.
var current atomic.Pointer[render.Model]

// reloadMu serializes the two writers of current (config watcher and status
// poller) so neither clobbers the other's half of the model.
var reloadMu sync.Mutex

// Store installs a model, and Current reads the live one.
func Store(m *render.Model) { current.Store(m) }

// Current returns the model every handler renders from.
func Current() *render.Model { return current.Load() }

// Serve builds the routes and blocks. cfgDir and assetsDir are served as
// static trees on top of the embedded ones.
func Serve(addr, cfgDir, assetsDir string) error {
	// ReadHeaderTimeout caps slow-header (Slowloris) clients; WriteTimeout
	// drops the ones that never read their answer; IdleTimeout reclaims
	// keep-alives. Every response is a small pre-rendered page, so these are
	// generous.
	srv := &http.Server{
		Addr:              addr,
		Handler:           mount(secureHeaders(routes(cfgDir, assetsDir))),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

// routes maps every path cairn answers. Nothing here is exported: what the
// program needs from this package is Serve, the loops and the probe.
func routes(cfgDir, assetsDir string) *http.ServeMux {
	mux := http.NewServeMux()
	static, _ := fs.Sub(render.Embedded, "assets")
	mux.Handle("GET /static/", cacheControl(http.StripPrefix("/static/", http.FileServerFS(static))))
	// Mounted unconditionally: a dir that appears after boot just works.
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", noListing(http.FileServer(http.Dir(assetsDir)))))
	// Service preview images live next to the yaml, in <config>/media/.
	mux.Handle("GET /media/", http.StripPrefix("/media/", noListing(http.FileServer(http.Dir(filepath.Join(cfgDir, "media"))))))
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz)
	mux.HandleFunc("GET /custom.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(cfgDir, "custom.css"))
	})
	mux.HandleFunc("GET /manifest.webmanifest", manifest)
	mux.HandleFunc("GET /robots.txt", robots)
	mux.HandleFunc("GET /sitemap.xml", sitemap)
	mux.HandleFunc("GET /{$}", root)
	// Catch-all rather than /{locale}/{$}: a wildcard pattern would conflict
	// with the /static/ and /assets/ subtrees in the 1.22 mux.
	mux.HandleFunc("GET /", home)
	return mux
}
