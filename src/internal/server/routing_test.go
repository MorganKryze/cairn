package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// The one that shipped broken, and the one a ResponseRecorder cannot catch:
// http.ServeContent calls WriteHeader before copying, so a middleware that
// decides at Write time mutates headers that have already gone out. Over a
// real connection the file then arrives gzipped with nothing saying so, and a
// browser runs the compressed bytes as JavaScript. Only a real server freezes
// headers the way this needs.
func TestCompressLabelsWhatItCompressesOverTheWire(t *testing.T) {
	js := strings.Repeat("function pad(){return 1}\n", 60)
	fsys := fstest.MapFS{"search.js": &fstest.MapFile{Data: []byte(js)}}
	srv := httptest.NewServer(compress(http.FileServerFS(fsys)))
	defer srv.Close()

	// Transport.DisableCompression stops Go from asking and unwrapping behind
	// our back, so the test sees exactly what a browser would.
	c := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	req, err := http.NewRequest("GET", srv.URL+"/search.js", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip: the body is compressed but nothing says so", got)
	}
	if got := resp.Header.Get("Content-Length"); got == strconv.Itoa(len(js)) {
		t.Errorf("Content-Length still claims the uncompressed %s", got)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("body is not a gzip stream: %v", err)
	}
	back, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != js {
		t.Error("the decompressed file is not the file on disk")
	}
}

// A 206 carries a Content-Range describing the uncompressed representation.
// Gzipping it produces a response that contradicts itself, and anything that
// fetches in slices reassembles garbage from it.
func TestCompressLeavesPartialContentAlone(t *testing.T) {
	css := strings.Repeat(":root{--accent:#247b7b}\n", 200)
	fsys := fstest.MapFS{"style.css": &fstest.MapFile{Data: []byte(css)}}
	srv := httptest.NewServer(compress(http.FileServerFS(fsys)))
	defer srv.Close()

	c := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	req, err := http.NewRequest("GET", srv.URL+"/style.css", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=100-199")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Errorf("a 206 came back with Content-Encoding %q; its Content-Range describes the uncompressed bytes", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 100 {
		t.Errorf("asked for 100 bytes, got %d", len(body))
	}
	if string(body) != css[100:200] {
		t.Error("the slice is not the bytes at that offset")
	}
}

// A HEAD must answer with the headers a GET would. Compressing it made the
// deferred Close emit an empty gzip member, and net/http advertised its 23
// bytes as the length of a 30 KB stylesheet.
func TestCompressAnswersHeadInIdentity(t *testing.T) {
	css := strings.Repeat(":root{--accent:#247b7b}\n", 200)
	fsys := fstest.MapFS{"style.css": &fstest.MapFile{Data: []byte(css)}}
	srv := httptest.NewServer(compress(http.FileServerFS(fsys)))
	defer srv.Close()

	c := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	req, err := http.NewRequest("HEAD", srv.URL+"/style.css", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Errorf("HEAD claimed Content-Encoding %q", got)
	}
	if got, want := resp.Header.Get("Content-Length"), strconv.Itoa(len(css)); got != want {
		t.Errorf("HEAD said Content-Length %q, want the real %q", got, want)
	}
}

// q=0 is how a client that cannot inflate says so. A substring search for
// "gzip" read it as a yes.
func TestCompressHonoursQualityZero(t *testing.T) {
	for _, c := range []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"gzip, deflate, br", true},
		{"gzip;q=0.8", true},
		{"deflate, gzip;q=1.0", true},
		{"GZIP", true},
		{"gzip;q=0", false},
		{"gzip;q=0, identity", false},
		{"notgzipatall", false},
		{"identity", false},
		{"", false},
	} {
		if got := acceptsGzip(c.header); got != c.want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", c.header, got, c.want)
		}
	}
}
