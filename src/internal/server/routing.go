package server

import (
	"compress/gzip"
	"io"
	"log"
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
		// Severs window.opener on the links that leave for a service, so a page
		// cairn opened cannot navigate the tab it came from. The template
		// already carries rel="noopener"; this is the belt that survives a
		// future edit forgetting it.
		hd.Set("Cross-Origin-Opener-Policy", "same-origin")
		// same-site rather than same-origin: self-hosters put their services on
		// sibling subdomains, and one of those pages showing a cairn icon is a
		// reasonable thing to do. A genuine third party still cannot embed
		// anything of ours.
		hd.Set("Cross-Origin-Resource-Policy", "same-site")
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

// gzipWriter decides at WriteHeader time, once the handler has settled its
// Content-Type. Deciding earlier would mean guessing from the path, which the
// /assets tree makes unreliable.
type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
}

// compressible covers what cairn actually serves as text. Images, fonts and
// the ico are already compressed; running them through gzip spends CPU to add
// bytes.
func compressible(ct string) bool {
	ct, _, _ = strings.Cut(ct, ";")
	switch strings.TrimSpace(ct) {
	// text/javascript rather than application/javascript: that is what Go's
	// mime table now hands back for a .js, and the older spelling alone let
	// search.js through uncompressed.
	case "text/html", "text/plain", "text/css", "text/xml", "text/javascript",
		"application/javascript", "application/json", "application/xml",
		"application/manifest+json", "image/svg+xml":
		return true
	}
	return false
}

// WriteHeader is where the decision has to be made, not Write: headers are
// frozen the moment it runs, and http.ServeContent calls it before copying a
// file. Deciding one write too late meant static files went out compressed
// with no Content-Encoding to say so, and a browser ran gzip bytes as
// JavaScript.
func (w *gzipWriter) WriteHeader(code int) {
	if !w.decided {
		w.decided = true
		// 2xx only: a 304 carries no body, and a compressed empty body is not
		// empty.
		if code/100 == 2 && compressible(w.Header().Get("Content-Type")) {
			// The length is the uncompressed one, and Go would send it as is.
			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")
			w.gz = gzip.NewWriter(w.ResponseWriter)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.decided {
		w.WriteHeader(http.StatusOK)
	}
	if w.gz == nil {
		return w.ResponseWriter.Write(b)
	}
	return w.gz.Write(b)
}

// ReadFrom has to exist, and has to funnel back through Write. The embedded
// http.ResponseWriter implements io.ReaderFrom, and embedding promotes it, so
// the io.Copy inside http.ServeContent would hand a file straight to the
// socket and skip the compression entirely. That is not hypothetical: the
// pages came back gzipped while every static file did not.
func (w *gzipWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{w}, r)
}

func (w *gzipWriter) Close() error {
	if w.gz != nil {
		return w.gz.Close()
	}
	return nil
}

// compress gzips the text responses for clients that asked. Vary rides on
// every response, compressed or not: a cache that stored the plain answer must
// not hand it to a client that would have got the compressed one, and the
// header is the only thing telling it so.
func compress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w}
		defer func() {
			if err := gw.Close(); err != nil {
				log.Printf("gzip: %v", err)
			}
		}()
		h.ServeHTTP(gw, r)
	})
}

func cacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		h.ServeHTTP(w, r)
	})
}
