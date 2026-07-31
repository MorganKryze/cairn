package server

import (
	"bytes"
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

// One validator cannot name two sets of bytes. The handler tags a page by its
// html, then this middleware changes the representation underneath it, so the
// tag has to carry the difference. Vary keeps a compliant cache honest; a cache
// told to ignore it hands gzip bytes to a client that asked for none, and
// nothing downstream can tell. Over a real server rather than a recorder,
// because the mark has to agree with the Content-Encoding beside it on the
// wire.
func TestETagNamesTheEncodingItWasSentWith(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	srv := httptest.NewServer(compress(http.HandlerFunc(home)))
	defer srv.Close()
	// DisableCompression stops Go asking and unwrapping behind our back, so the
	// test sees exactly what a browser would.
	c := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	get := func(t *testing.T, enc, inm string) *http.Response {
		t.Helper()
		req, err := http.NewRequest("GET", srv.URL+"/en/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", enc)
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	plain, zipped := get(t, "identity", ""), get(t, "gzip", "")
	if got := plain.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("the identity answer carried Content-Encoding %q", got)
	}
	if got := zipped.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	identityTag, gzipTag := plain.Header.Get("ETag"), zipped.Header.Get("ETag")
	if identityTag == "" || gzipTag == "" {
		t.Fatalf("tags = %q and %q, want one on each answer", identityTag, gzipTag)
	}
	if identityTag == gzipTag {
		t.Fatalf("both representations answer to %s: a cache that ignores Vary can hand the gzip bytes to a client that cannot inflate them", gzipTag)
	}
	// The mark is a mark, not a new tag: the identity one still has to be
	// readable inside it, or a proxy comparing the two learns nothing.
	if !strings.HasPrefix(gzipTag, strings.TrimSuffix(identityTag, `"`)) {
		t.Errorf("gzip tag %s is not the identity tag %s with a mark on it", gzipTag, identityTag)
	}

	// The trap: the handler compares If-None-Match before this layer would
	// rewrite anything, so a marked tag has to be unmarked on the way in or the
	// second visit to an unchanged page downloads it again.
	back := get(t, "gzip", gzipTag)
	if back.StatusCode != http.StatusNotModified {
		t.Errorf("revalidating the compressed page = %d, want 304: the mark is not coming back off on the way in", back.StatusCode)
	}
	if got := back.Header.Get("ETag"); got != gzipTag {
		t.Errorf("the 304 named %s, want the %s the client is still holding", got, gzipTag)
	}

	// And the other half of it: the same client after a browser update that
	// dropped gzip is holding compressed bytes it can no longer read. Unmarking
	// unconditionally would answer 304 and leave it stuck with them.
	stale := get(t, "identity", gzipTag)
	if stale.StatusCode != http.StatusOK {
		t.Fatalf("a gzip tag presented without gzip = %d, want 200: the client needs the identity bytes", stale.StatusCode)
	}
	if got := stale.Header.Get("ETag"); got != identityTag {
		t.Errorf("the identity answer named %s, want %s", got, identityTag)
	}

	// An identity cache revalidating is untouched by any of it.
	if got := get(t, "identity", identityTag).StatusCode; got != http.StatusNotModified {
		t.Errorf("revalidating the plain page = %d, want 304", got)
	}

	// Whatever the tag says, the bytes still have to be the page.
	fresh := get(t, "gzip", "")
	zr, err := gzip.NewReader(fresh.Body)
	if err != nil {
		t.Fatalf("body is not a gzip stream: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, current.Load().Pages["en"].HTML) {
		t.Error("the decompressed page is not the page that was rendered")
	}
}

// A weak tag is a tag, and plenty of caches hold one. Marking it must leave it
// weak, and unmarking must not reach into an opaque value that happens to read
// like a mark.
func TestGzipMarkSurvivesEveryShapeOfTag(t *testing.T) {
	for _, c := range []struct{ plain, marked string }{
		{`"9f86d081884c7d65"`, `"9f86d081884c7d65-gzip"`},
		{`W/"9f86d081884c7d65"`, `W/"9f86d081884c7d65-gzip"`},
		{`""`, `"-gzip"`},
	} {
		if got := markGzip(c.plain); got != c.marked {
			t.Errorf("markGzip(%s) = %s, want %s", c.plain, got, c.marked)
		}
		if got := unmarkGzip(c.marked); got != c.plain {
			t.Errorf("unmarkGzip(%s) = %s, want %s", c.marked, got, c.plain)
		}
	}

	for _, c := range []struct{ in, want string }{
		// A list, marked or half marked, comes back tag for tag.
		{`"a-gzip", W/"b-gzip"`, `"a", W/"b"`},
		{`"a-gzip", "b"`, `"a", "b"`},
		// Nothing to do, so nothing is done.
		{`*`, `*`},
		{`"a", W/"b"`, `"a", W/"b"`},
		{``, ``},
		// An entity-tag may hold a comma and may end in the word itself; only a
		// mark meeting its own closing quote is one.
		{`"a,b-gzip"`, `"a,b"`},
		{`"gzip"`, `"gzip"`},
		{`"a-gzip-still"`, `"a-gzip-still"`},
	} {
		if got := unmarkGzip(c.in); got != c.want {
			t.Errorf("unmarkGzip(%s) = %s, want %s", c.in, got, c.want)
		}
	}

	// Nothing that is not a quoted tag is touched, rather than being turned
	// into something that parses as neither.
	if got := markGzip("*"); got != "*" {
		t.Errorf("markGzip(*) = %s, want it left alone", got)
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
