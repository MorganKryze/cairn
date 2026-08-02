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
		Handler:           handler(cfgDir, assetsDir),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

// handler is the whole chain, written outside in.
//
// secureHeaders is outermost because that is the only place nothing can answer
// from underneath it. mount registers three things of its own at the domain
// root, /healthz, /readyz and the 404 for everything outside the prefix, and
// with the headers inside mount those three came back bare under -base-path:
// no COOP, no CORP, no HSTS. HSTS is the one that costs, because the bare host
// is exactly where a first visit lands, and a header that never arrives there
// cannot pin the scheme before someone gets to downgrade it.
//
// compress stays inside secureHeaders and still outside mount, so the two
// deployments answer alike. Without -base-path the probes were already inside
// both wrappers and nobody chose that difference; putting mount innermost
// settles it in the direction the default deployment has been running all
// along. It costs the probes nothing: healthz and readyz set no Content-Type,
// compress reads the one on the header before net/http gets to sniff one, and
// an empty type is not compressible, so "ok\n" goes out as three bytes either
// way. What the two wrappers actually add there is Vary and the hardening
// headers, which is the whole point.
func handler(cfgDir, assetsDir string) http.Handler {
	return secureHeaders(compress(mount(routes(cfgDir, assetsDir))))
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
	// And the self-hosted font theme.font.file names, in <config>/fonts/.
	mux.Handle("GET /fonts/", http.StripPrefix("/fonts/", noListing(http.FileServer(http.Dir(filepath.Join(cfgDir, "fonts"))))))
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz)
	mux.HandleFunc("GET /custom.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(cfgDir, "custom.css"))
	})
	mux.HandleFunc("GET /manifest.webmanifest", manifest)
	mux.Handle("GET /favicon.ico", cacheControl(http.HandlerFunc(faviconICO(static))))
	mux.HandleFunc("GET /robots.txt", robots)
	mux.HandleFunc("GET /sitemap.xml", sitemap)
	mux.HandleFunc("GET /.well-known/security.txt", securityTxt)
	mux.HandleFunc("GET /{$}", root)
	// Catch-all rather than /{locale}/{$}: a wildcard pattern would conflict
	// with the /static/ and /assets/ subtrees in the 1.22 mux.
	mux.HandleFunc("GET /", home)
	return mux
}
