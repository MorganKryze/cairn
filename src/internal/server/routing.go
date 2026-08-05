package server

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/MorganKryze/cairn/src/internal/render"
)

// mount puts the whole site under render.BasePath. The two probes stay at the
// root whatever the prefix: an orchestrator talks to cairn directly, not
// through the proxy that adds it.
func mount(h http.Handler) http.Handler {
	if render.BasePath == "" {
		return h
	}
	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", healthz)
	outer.HandleFunc("GET /readyz", readyz)
	// Registering the subtree also makes ServeMux redirect a bare /cairn to
	// /cairn/, so the mount point works with or without its trailing slash.
	outer.Handle(render.BasePath+"/", http.StripPrefix(render.BasePath, h))
	return outer
}

// secureHeaders sets the hardening headers. The CSP whitelists exactly what
// the pages use; see BuildCSP.
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
		// already carries rel="noopener"; this survives an edit that drops it.
		hd.Set("Cross-Origin-Opener-Policy", "same-origin")
		// same-site rather than same-origin: self-hosters put their services on
		// sibling subdomains, and one of those pages may show a cairn icon. A
		// genuine third party still cannot embed anything of ours.
		hd.Set("Cross-Origin-Resource-Policy", "same-site")
		// HSTS only once the visit is already https, like the cookies: sending
		// it over plain http would strand the LAN deployments cairn also serves.
		if secureRequest(r) {
			hd.Set("Strict-Transport-Security", "max-age=31536000")
		}
		h.ServeHTTP(w, r)
	})
}

// hasDotSegment covers a .. that survived the mux's cleaning as well as the
// dotfiles: the dot can be anywhere in the path, not only at the front.
func hasDotSegment(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func noListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// After StripPrefix the folder root is the empty path.
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		// -assets gets pointed at a working copy often enough that it has to be
		// safe: with the dot served, /assets/.git/config is a public URL
		// carrying the remote and whatever a credential helper wrote into it.
		// The whole class goes rather than a blocklist, which stays one .env or
		// .DS_Store behind. Nothing cairn serves lives under a dot:
		// /.well-known/security.txt has its own handler on the route table,
		// never out of these trees.
		if hasDotSegment(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// gzipPool keeps the compressor's window and tables between requests. A fresh
// gzip.NewWriter costs about 800 KB of heap, held for as long as the client
// takes to read: 200 slow readers took a process from 16 MB to 171 MB, past
// the 64Mi the chart asks for and the 128M the compose file sets. Pooled, that
// cost is paid once per concurrent response.
var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// gzipMark separates the compressed representation's validator from the
// identity one's. The handler tags a page by its html alone, so without this
// one ETag names two different answers; Vary keeps a compliant cache honest,
// but a cache configured to ignore it then hands gzip bytes to a client that
// cannot inflate them.
//
// The mark goes inside the quotes. An entity-tag is a quoted string and
// nothing but, so Apache's historical `"…"-gzip` is not a valid one: a strict
// parser drops the tag and revalidation stops happening at all.
const gzipMark = "-gzip"

// markGzip inserts the mark before the closing quote rather than appending it,
// so a weak `W/"…"` comes back weak instead of parsing as neither.
func markGzip(tag string) string {
	if !strings.HasSuffix(tag, `"`) {
		return tag
	}
	return tag[:len(tag)-1] + gzipMark + `"`
}

// unmarkGzip is the inverse, over a whole If-None-Match list. A double quote
// is the one byte an entity-tag's body may not contain, so `-gzip"` in a
// well-formed list is always a mark of ours meeting its closing quote, never a
// slice out of someone's opaque value. One pass, without splitting on a comma
// an entity-tag is allowed to contain.
func unmarkGzip(list string) string {
	return strings.ReplaceAll(list, gzipMark+`"`, `"`)
}

type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
	// marked: the request arrived carrying a marked validator, which is the
	// only thing a 304 has left to reconstruct one from.
	marked bool
}

func compressible(ct string) bool {
	ct, _, _ = strings.Cut(ct, ";")
	switch strings.TrimSpace(ct) {
	// text/javascript rather than application/javascript: that is what Go's
	// mime table hands back for a .js, and the older spelling alone let
	// search.js through uncompressed.
	case "text/html", "text/plain", "text/css", "text/xml", "text/javascript",
		"application/javascript", "application/json", "application/xml",
		"application/manifest+json", "image/svg+xml":
		return true
	}
	return false
}

// WriteHeader is where the decision has to be made, not Write: the handler has
// settled its Content-Type by now, headers freeze the moment WriteHeader runs,
// and http.ServeContent calls it before copying a file. One write too late,
// static files went out compressed with no Content-Encoding to say so, and a
// browser ran gzip bytes as JavaScript.
func (w *gzipWriter) WriteHeader(code int) {
	if !w.decided {
		w.decided = true
		// 200 only, not any 2xx. A 206 carries a Content-Range describing the
		// uncompressed representation, so gzipping it reassembles into garbage
		// for anything that fetches in slices.
		if code == http.StatusOK && compressible(w.Header().Get("Content-Type")) {
			// The length is the uncompressed one, and Go would send it as is.
			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")
			w.gz = gzipPool.Get().(*gzip.Writer)
			w.gz.Reset(w.ResponseWriter)
		}
		// A 304 sends no bytes, so the representation being confirmed is the one
		// the client already holds: the right tag there is the one it sent, so
		// the mark goes back on exactly when it came off. Any other way hands an
		// identity cache a gzip tag, or the reverse, and the next revalidation
		// is wrong.
		if w.gz != nil || (code == http.StatusNotModified && w.marked) {
			if tag := w.Header().Get("ETag"); tag != "" {
				w.Header().Set("ETag", markGzip(tag))
			}
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
// socket and skip compression: the pages came back gzipped while every static
// file did not.
func (w *gzipWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{w}, r)
}

func (w *gzipWriter) Close() error {
	if w.gz == nil {
		return nil
	}
	err := w.gz.Close()
	// Reset before returning it: a pooled writer still holding this response's
	// ResponseWriter would keep the whole request alive.
	w.gz.Reset(io.Discard)
	gzipPool.Put(w.gz)
	w.gz = nil
	return err
}

// acceptsGzip parses the header rather than searching it for a substring.
// "gzip;q=0" is how a client that cannot inflate says so, and a substring
// search took it for a yes; "notgzipatall" is not an offer at all.
func acceptsGzip(h string) bool {
	for _, part := range strings.Split(h, ",") {
		tok, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(tok), "gzip") {
			continue
		}
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if q, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && q == 0 {
				return false
			}
		}
		return true
	}
	return false
}

// compress gzips the text responses for clients that asked. Vary rides on
// every response, compressed or not: a cache that stored the plain answer must
// not hand it to a client that would have got the compressed one, and the
// header is the only thing telling it so. The validator says the same thing a
// second way for caches told to ignore Vary; see gzipMark.
func compress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		// A HEAD has to answer with the headers a GET would, and the only way
		// to state a compressed length is to compress a body that is then
		// thrown away. Answered in identity instead: the headers then describe
		// the uncompressed representation, which is a true one, rather than
		// advertising the 23 bytes of an empty gzip member.
		if r.Method == http.MethodHead || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			// The mark stays on the way in. A client that cached the compressed
			// page and can no longer inflate one needs the identity bytes, so
			// its marked tag must fail the handler's comparison and earn a 200;
			// stripping it would answer 304 and leave it holding gzip it cannot
			// read.
			h.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w}
		// The handler runs its own If-None-Match comparison before this layer
		// would rewrite anything on the way out, so the mark has to come off
		// here or every conditional request on a compressed page comes back as
		// a full 200. Copied rather than edited in place: a handler is not
		// supposed to find its request mutated underneath it.
		if inm := r.Header.Get("If-None-Match"); strings.Contains(inm, gzipMark+`"`) {
			gw.marked = true
			clone := *r
			clone.Header = r.Header.Clone()
			clone.Header.Set("If-None-Match", unmarkGzip(inm))
			r = &clone
		}
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

// stamped serves cairn's own assets under the digested names the pages point
// at, and only under a digest cairn itself issued. A year and immutable is
// safe for one reason: the name changes whenever the bytes do, so this
// response can never become the wrong answer. The pages that point here are
// served no-cache, so an upgrade is picked up on the next visit without a hard
// refresh.
//
// A plain name falls through to the handler behind this one and its day-long
// cache, so anything that hardcoded /static/style.css goes on working. A name
// shaped like a digest cairn never issued falls through too, and 404s.
func stamped(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if real, ok := render.AssetPath(strings.TrimPrefix(r.URL.Path, "/static/")); ok {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			r2 := new(http.Request)
			*r2 = *r
			u := *r.URL
			u.Path = "/static/" + real
			r2.URL = &u
			h.ServeHTTP(w, r2)
			return
		}
		h.ServeHTTP(w, r)
	})
}
