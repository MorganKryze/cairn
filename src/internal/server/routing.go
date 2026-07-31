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

// noListing serves files but never directory indexes, and never anything under
// a dot-prefixed name.
func noListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// After StripPrefix the folder root is the empty path.
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		// -assets gets pointed at a working copy often enough that it has to be
		// safe: with the dot served, /assets/.git/config is a public URL, and it
		// carries the remote and whatever a credential helper wrote into it.
		// The whole class goes rather than a blocklist, which would be one .env
		// or .DS_Store behind for as long as the file exists. Nothing cairn
		// serves lives under a dot: the icons, logos and previews are ordinary
		// names, and /.well-known/security.txt is answered by its own handler on
		// the route table, never out of these trees. Segment by segment because
		// the dot can be anywhere in the path, and this catches a .. that
		// somehow survived the mux's cleaning on the way here too.
		for _, seg := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(seg, ".") {
				http.NotFound(w, r)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// gzipPool keeps the compressor's window and tables between requests. A fresh
// gzip.NewWriter costs about 800 KB of heap, held for as long as the client
// takes to read: measured, 200 slow readers took a process from 16 MB to 171
// MB, which crosses the 64Mi the chart asks for by default and the 128M the
// compose file sets. Pooled, that cost is paid once per concurrent response
// rather than once per response.
var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// gzipMark separates the compressed representation's validator from the
// identity one's. The handler tags a page by its html alone, so without this
// one ETag names two different answers; Vary keeps a compliant cache honest,
// but a cache configured to ignore it then hands gzip bytes to a client that
// cannot inflate them, and the client has no way to tell.
//
// The mark goes inside the quotes. An entity-tag is a quoted string and
// nothing but, so Apache's historical `"…"-gzip`, with the mark hung off the
// outside, is not a valid one; a strict parser drops the tag and revalidation
// quietly stops happening at all.
const gzipMark = "-gzip"

// markGzip moves one validator onto the compressed representation. Inserted
// before the closing quote rather than appended, so a weak `W/"…"` comes back
// weak instead of turning into something that parses as neither.
func markGzip(tag string) string {
	if !strings.HasSuffix(tag, `"`) {
		return tag
	}
	return tag[:len(tag)-1] + gzipMark + `"`
}

// unmarkGzip is the inverse, over a whole If-None-Match list. ReplaceAll needs
// no parsing and cannot corrupt a tag: a double quote is the one byte an
// entity-tag's body may not contain, so `-gzip"` in a well-formed list is
// always a mark of ours meeting its closing quote, never a slice out of the
// middle of someone's opaque value. A bare `*` and every unmarked tag come
// through untouched, and a list is handled in one pass without splitting on a
// comma, which an entity-tag is allowed to contain.
func unmarkGzip(list string) string {
	return strings.ReplaceAll(list, gzipMark+`"`, `"`)
}

// gzipWriter decides at WriteHeader time, once the handler has settled its
// Content-Type. Deciding earlier would mean guessing from the path, which the
// /assets tree makes unreliable.
type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
	// marked records that the request arrived carrying a marked validator,
	// which is the only thing a 304 has left to reconstruct one from.
	marked bool
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
		// 200 only, not any 2xx. A 206 carries a Content-Range describing the
		// uncompressed representation, so gzipping it produces a response that
		// contradicts itself and reassembles into garbage for anything that
		// fetches in slices.
		if code == http.StatusOK && compressible(w.Header().Get("Content-Type")) {
			// The length is the uncompressed one, and Go would send it as is.
			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")
			w.gz = gzipPool.Get().(*gzip.Writer)
			w.gz.Reset(w.ResponseWriter)
		}
		// A validator names the bytes that go out, and this is the line that
		// settled which ones those are. A 304 sends none, so the representation
		// being confirmed is the one the client already holds: the right tag
		// there is the one it sent, which means the mark goes back on exactly
		// when it came off. Doing it any other way hands an identity cache a
		// gzip tag, or the reverse, and the next revalidation is wrong.
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
// socket and skip the compression entirely. That is not hypothetical: the
// pages came back gzipped while every static file did not.
func (w *gzipWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{w}, r)
}

func (w *gzipWriter) Close() error {
	if w.gz == nil {
		return nil
	}
	err := w.gz.Close()
	// Reset before returning it: a writer put back still holding a reference to
	// this response's ResponseWriter would keep the whole request alive.
	w.gz.Reset(io.Discard)
	gzipPool.Put(w.gz)
	w.gz = nil
	return err
}

// acceptsGzip reads the header rather than searching it for a substring.
// "gzip;q=0" is how a client that cannot inflate says so, and it used to be
// taken for a yes; "notgzipatall" is not an offer at all.
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
// header is the only thing telling it so. The validator is marked to say the
// same thing a second way, for the caches that are told to ignore Vary; see
// gzipMark.
func compress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		// A HEAD has to answer with the headers a GET would, and the only way
		// to state a compressed length is to compress a body that is then
		// thrown away. Answered in identity instead: the headers then describe
		// the uncompressed representation, which is a true one, rather than
		// advertising the 23 bytes of an empty gzip member.
		if r.Method == http.MethodHead || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			// The mark is deliberately left on the way in here. A client that
			// cached the compressed page and can no longer inflate one needs
			// the identity bytes, so its marked tag must fail the handler's
			// comparison and earn a 200; stripping it unconditionally would
			// answer 304 and leave it holding gzip it cannot read.
			h.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w}
		// The handler does its own If-None-Match comparison, and it does it
		// before this layer would rewrite anything on the way out, so the mark
		// has to come off here or every conditional request on a compressed
		// page comes back as a full 200 and the ETag stops saving a byte.
		// Copied rather than edited in place: a handler is not supposed to find
		// its request mutated underneath it, and the copy is only paid for by
		// the requests that actually carry a mark.
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
