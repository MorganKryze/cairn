package status_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/status"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestEmitGatus(t *testing.T) {
	out, err := status.Emit(sample(t))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"name: pdf", "group: documents", "url: https://pdf.example.org", "name: pad", "[STATUS] == 200"} {
		if !strings.Contains(s, want) {
			t.Errorf("emitted config missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "group: \"\"") {
		t.Error("service without category should omit group")
	}
}

func TestFetchStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/endpoints/statuses" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `[
			{"name":"pdf","results":[{"success":false},{"success":true}]},
			{"name":"pad","results":[{"success":false}]},
			{"name":"empty","results":[]}
		]`)
	}))
	defer srv.Close()
	st, err := status.Fetch(srv.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if up, ok := st["pdf"]; !ok || !up.Up {
		t.Errorf("pdf = %v,%v, want up (last result wins)", up, ok)
	}
	if up, ok := st["pad"]; !ok || up.Up {
		t.Errorf("pad = %v,%v, want down", up, ok)
	}
	if _, ok := st["empty"]; ok {
		t.Error("endpoint without results should be unknown")
	}
}

// Gatus names its own endpoint pages, and only the operator's Gatus config
// decides how endpoints are grouped. cairn used to build that URL out of its
// own category, which is right only for someone who ran -emit-gatus and kept
// the groups: anyone who wrote their endpoints by hand got a pill pointing at
// a page that does not exist.
func TestPillLinksToTheKeyGatusReports(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ungrouped endpoints, though cairn files both services under a category.
		io.WriteString(w, `[
			{"name":"pdf","key":"_pdf","results":[{"success":true}]},
			{"name":"pad","key":"_pad","results":[{"success":true}]}
		]`)
	}))
	defer srv.Close()
	st, err := status.Fetch(srv.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := st["pdf"].Key; got != "_pdf" {
		t.Errorf("Fetch dropped the reported key: pdf key = %q, want %q", got, "_pdf")
	}

	m, err := render.BuildModel(sample(t), st)
	if err != nil {
		t.Fatal(err)
	}
	html := string(m.Pages["fr"].HTML)
	if !strings.Contains(html, `href="https://status.example.org/endpoints/_pdf"`) {
		t.Error("pill does not link to the key gatus reported")
	}
	if strings.Contains(html, "endpoints/documents_pdf") {
		t.Error("pill still links to a key cairn guessed from its own category")
	}
}

// A key is a path segment cairn did not write, coming off the network from a
// service that could be anything answering on that address.
func TestReportedKeyCannotEscapeItsPathSegment(t *testing.T) {
	m, err := render.BuildModel(sample(t), map[string]status.State{
		"pdf": {Up: true, Key: "../../../admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if html := string(m.Pages["fr"].HTML); strings.Contains(html, "endpoints/../") {
		t.Error("a reported key climbed out of the endpoints path")
	}
}

func TestStatusDots(t *testing.T) {
	cfg := sample(t)
	m, err := render.BuildModel(cfg, map[string]status.State{"pdf": {Up: true}, "pad": {}})
	if err != nil {
		t.Fatal(err)
	}
	html := string(m.Pages["fr"].HTML)
	if !strings.Contains(html, "status-up") || !strings.Contains(html, "status-down") {
		t.Error("home missing status pills")
	}
	if !strings.Contains(html, "En ligne") || !strings.Contains(html, "Hors ligne") {
		t.Error("pills missing localized labels")
	}
	if !strings.Contains(html, `href="https://status.example.org/endpoints/documents_pdf"`) {
		t.Error("pdf pill should link to its own endpoint page")
	}
	if !strings.Contains(html, `href="https://status.example.org/endpoints/_pad"`) {
		t.Error("uncategorized service should link to the ungrouped endpoint key")
	}

	pubDir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "status: {gatus: http://gatus:8080, page: https://status.example.org}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	pub, err := config.Load(pubDir)
	if err != nil {
		t.Fatal(err)
	}
	pm, err := render.BuildModel(pub, map[string]status.State{"a": {Up: true}})
	if err != nil {
		t.Fatal(err)
	}
	ph2 := string(pm.Pages["en"].HTML)
	if !strings.Contains(ph2, `href="https://status.example.org/endpoints/_a"`) || strings.Contains(ph2, "gatus:8080") {
		t.Error("pill should link to status.page, not the internal poll URL")
	}

	pending, err := render.BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ph := string(pending.Pages["fr"].HTML)
	if strings.Count(ph, "status-unknown") != 2 || !strings.Contains(ph, "Inconnu") {
		t.Error("gatus configured but silent: every pill should be unknown")
	}

	partial, err := render.BuildModel(cfg, map[string]status.State{"pdf": {Up: true}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(partial.Pages["fr"].HTML), "status-pill") != 1 {
		t.Error("service unknown to gatus should show no pill once gatus answered")
	}

	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: solo, url: https://solo.example.org, name: Solo}\n",
	})
	noGatus, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	off, err := render.BuildModel(noGatus, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(off.Pages["en"].HTML), "status-pill") {
		t.Error("pills rendered although status.gatus is not configured")
	}
}

func TestStatusConfigValidation(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "status: {gatus: status.example.org}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := config.Load(dir); err == nil || !strings.Contains(err.Error(), "status.gatus") {
		t.Errorf("error = %v, want status.gatus complaint", err)
	}
	dir = testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "status: {gatus: https://status.example.org, interval: 1s}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := config.Load(dir); err == nil || !strings.Contains(err.Error(), "status.interval") {
		t.Errorf("error = %v, want status.interval complaint", err)
	}
}

func TestUnmonitored(t *testing.T) {
	cfg := &config.Config{Categories: []config.Category{{ID: "t", Services: []config.Service{{ID: "seen"}, {ID: "ghost"}}}}}
	if got := status.Unmonitored(cfg, map[string]status.State{"seen": {Up: true}}); !strings.Contains(got, "ghost") || strings.Contains(got, "seen,") {
		t.Errorf("Unmonitored = %q, want it to name only ghost", got)
	}
	if got := status.Unmonitored(cfg, map[string]status.State{"seen": {Up: true}, "ghost": {}}); got != "" {
		t.Errorf("Unmonitored = %q, want empty when all endpoints exist", got)
	}
}
