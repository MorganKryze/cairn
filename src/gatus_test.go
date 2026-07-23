package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testConfig(t *testing.T) *Config {
	t.Helper()
	dir := writeFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nstatus: {gatus: https://status.example.org}\n",
		"services.yaml": "- {id: pdf, url: https://pdf.example.org, category: documents, name: PDF}\n" +
			"- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestEmitGatus(t *testing.T) {
	out, err := emitGatus(testConfig(t))
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
	st, err := fetchStatuses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if up, ok := st["pdf"]; !ok || !up {
		t.Errorf("pdf = %v,%v, want up (last result wins)", up, ok)
	}
	if up, ok := st["pad"]; !ok || up {
		t.Errorf("pad = %v,%v, want down", up, ok)
	}
	if _, ok := st["empty"]; ok {
		t.Error("endpoint without results should be unknown")
	}
}

func TestStatusDots(t *testing.T) {
	cfg := testConfig(t)
	m, err := buildModel(cfg, map[string]bool{"pdf": true, "pad": false})
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

	pubDir := writeFiles(t, map[string]string{
		"site.yaml":     "status: {gatus: http://gatus:8080, page: https://status.example.org}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	pub, err := loadConfig(pubDir)
	if err != nil {
		t.Fatal(err)
	}
	pm, err := buildModel(pub, map[string]bool{"a": true})
	if err != nil {
		t.Fatal(err)
	}
	ph2 := string(pm.Pages["en"].HTML)
	if !strings.Contains(ph2, `href="https://status.example.org/endpoints/_a"`) || strings.Contains(ph2, "gatus:8080") {
		t.Error("pill should link to status.page, not the internal poll URL")
	}

	pending, err := buildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ph := string(pending.Pages["fr"].HTML)
	if strings.Count(ph, "status-unknown") != 2 || !strings.Contains(ph, "Inconnu") {
		t.Error("gatus configured but silent: every pill should be unknown")
	}

	partial, err := buildModel(cfg, map[string]bool{"pdf": true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(partial.Pages["fr"].HTML), "status-pill") != 1 {
		t.Error("service unknown to gatus should show no pill once gatus answered")
	}

	dir := writeFiles(t, map[string]string{
		"services.yaml": "- {id: solo, url: https://solo.example.org, name: Solo}\n",
	})
	noGatus, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	off, err := buildModel(noGatus, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(off.Pages["en"].HTML), "status-pill") {
		t.Error("pills rendered although status.gatus is not configured")
	}
}

func TestStatusConfigValidation(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml":     "status: {gatus: status.example.org}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "status.gatus") {
		t.Errorf("error = %v, want status.gatus complaint", err)
	}
	dir = writeFiles(t, map[string]string{
		"site.yaml":     "status: {gatus: https://status.example.org, interval: 1s}\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "status.interval") {
		t.Errorf("error = %v, want status.interval complaint", err)
	}
}
