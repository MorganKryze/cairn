package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// compress has to stay invisible: the bytes a client gets back must be the
// bytes the handler wrote, whatever the wire carried.
func TestCompressRoundTrips(t *testing.T) {
	body := strings.Repeat("cairn sert des pages calmes. ", 40)
	h := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, body)
	}))

	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/en/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, r)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if rec.Body.Len() >= len(body) {
		t.Errorf("compressed %d bytes into %d, which is not compression", len(body), rec.Body.Len())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	back, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != body {
		t.Error("what came back out is not what the handler wrote")
	}
}

// A client that did not ask gets plain bytes, and every response says the
// answer depends on Accept-Encoding so a shared cache cannot mix the two up.
func TestCompressLeavesUnaskedClientsAlone(t *testing.T) {
	h := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "plain")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/en/", nil))

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("unasked client got Content-Encoding %q", got)
	}
	if rec.Body.String() != "plain" {
		t.Errorf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Errorf("Vary = %q, want it to mention Accept-Encoding", got)
	}
}

// Images, fonts and the ico are already compressed. Running them through gzip
// spends CPU to add bytes, so the middleware has to leave them be.
// Every content type cairn actually answers with has to be covered, or a file
// slips through uncompressed and nothing says so. text/javascript is the one
// that did: the older application/javascript spelling is not what Go serves.
func TestCompressCoversEveryTextTypeCairnServes(t *testing.T) {
	for _, ct := range []string{
		"text/html; charset=utf-8",
		"text/css; charset=utf-8",
		"text/javascript; charset=utf-8",
		"text/plain; charset=utf-8",
		"application/xml; charset=utf-8",
		"application/manifest+json",
		"image/svg+xml",
	} {
		if !compressible(ct) {
			t.Errorf("%s is served by cairn but never compressed", ct)
		}
	}
}

func TestCompressSkipsAlreadyCompressedTypes(t *testing.T) {
	for _, ct := range []string{"image/png", "font/woff2", "image/x-icon"} {
		h := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", ct)
			io.WriteString(w, strings.Repeat("\x00", 500))
		}))
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/static/x", nil)
		r.Header.Set("Accept-Encoding", "gzip")
		h.ServeHTTP(rec, r)
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("%s was compressed (Content-Encoding %q)", ct, got)
		}
	}
}

// A 304 carries no body. Compressing one would mean sending a gzip header
// where the client expects nothing at all.
func TestCompressLeavesNotModifiedEmpty(t *testing.T) {
	h := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotModified)
	}))
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/en/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("304 carried %d bytes", rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("304 claimed Content-Encoding %q", got)
	}
}

// The uncompressed length would be a lie once the body is gzipped, and a
// client that trusts it hangs waiting for bytes that never come.
func TestCompressDropsStaleContentLength(t *testing.T) {
	body := strings.Repeat("x", 400)
	h := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Header().Set("Content-Length", "400")
		io.WriteString(w, body)
	}))
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/static/style.css", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, r)

	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Errorf("Content-Length survived as %q", got)
	}
}

// http.ServeContent copies through io.Copy, which uses io.ReaderFrom when the
// writer offers one. Embedding http.ResponseWriter offers one, so without our
// own ReadFrom the file goes straight to the socket and the compression is
// silently skipped. This is the case that actually shipped broken: pages came
// back gzipped, every static file did not.
func TestCompressSurvivesServeContent(t *testing.T) {
	css := strings.Repeat(":root{--accent:#247b7b}\n", 60)
	fsys := fstest.MapFS{"style.css": &fstest.MapFile{Data: []byte(css)}}
	h := compress(http.FileServerFS(fsys))

	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/style.css", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, r)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("a file served through ServeContent came back with Content-Encoding %q, want gzip", got)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	back, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != css {
		t.Error("the decompressed file is not the file on disk")
	}
}
