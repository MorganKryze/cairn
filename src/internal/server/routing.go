package server

import (
	"net/http"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/render"
)

// mount puts the whole site under render.BasePath. The two probes stay at the root
// whatever the prefix: an orchestrator talks to cairn directly, not through
// the proxy that adds it.
func mount(h http.Handler) http.Handler {
	if render.BasePath == "" {
		return h
	}
	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", healthz)
	outer.HandleFunc("GET /readyz", readyz)
	// registering the subtree also makes ServeMux redirect a bare /cairn to
	// /cairn/, so the mount point works with or without its trailing slash
	outer.Handle(render.BasePath+"/", http.StripPrefix(render.BasePath, h))
	return outer
}

// secureHeaders sets the response headers a security scan would ask for.
// The CSP whitelists exactly what the pages use; see BuildCSP.
func secureHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		hd.Set("Content-Security-Policy", Current().CSP)
		// HSTS only once the visit is already https, like the cookies: sending
		// it over plain http would strand the LAN deployments cairn also serves.
		if secureRequest(r) {
			hd.Set("Strict-Transport-Security", "max-age=31536000")
		}
		h.ServeHTTP(w, r)
	})
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
