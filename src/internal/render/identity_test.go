package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestFaviconTaglineOGImageNoindex(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlogo: /assets/logo.png\nfavicon: /assets/fav.png\n" +
			"index: false\ntagline: Small tools, kindly hosted\nlocales: [en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	home := string(m.Pages["en"].HTML)
	for _, want := range []string{
		`<link rel="icon" href="/assets/fav.png">`,
		`<meta property="og:image" content="https://tools.example.org/assets/logo.png">`,
		`<meta name="robots" content="noindex">`,
		`<p class="tagline">Small tools, kindly hosted</p>`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("home is missing %s", want)
		}
	}
	if strings.Contains(home, "favicon.svg") {
		t.Error("default favicon should be replaced")
	}
}

func TestOGImage(t *testing.T) {
	for _, tc := range []struct{ base, logo, want string }{
		{"https://x.org", "/assets/l.png", "https://x.org/assets/l.png"},
		{"https://x.org", "https://cdn.example.org/l.jpg", "https://cdn.example.org/l.jpg"},
		{"https://x.org", "/assets/l.svg", ""},
		{"", "/assets/l.png", ""},
		{"https://x.org", "", ""},
	} {
		if got := ogImage(tc.base, tc.logo); got != tc.want {
			t.Errorf("ogImage(%q, %q) = %q, want %q", tc.base, tc.logo, got, tc.want)
		}
	}
}

// The og:image is built from the public url and a logo AppURL has already
// resolved. Adding the base path on both sides sent every social preview to
// /cairn/cairn/…, which 404s, while the header logo on the same page was
// right. Only a run with -base-path set could show it.
func TestOGImageCarriesTheBasePathOnce(t *testing.T) {
	old := BasePath
	BasePath = "/cairn"
	defer func() { BasePath = old }()

	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "url: https://tools.example.org\nlocales: [en]\nlogo: /assets/logo.png\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	html := string(m.Pages["en"].HTML)

	const want = `content="https://tools.example.org/cairn/assets/logo.png"`
	if !strings.Contains(html, want) {
		t.Errorf("og:image is not %s\ngot: %s", want, ogLine(html))
	}
	if strings.Contains(html, "/cairn/cairn/") {
		t.Errorf("the base path landed twice: %s", ogLine(html))
	}
}

// A base path with no public url used to make Base a bare "/cairn", which the
// template reads as truthy, so the page emitted relative canonical, og:url and
// hreflang links. Emitting none is the documented meaning of an unset url.
func TestNoPublicURLEmitsNoSelfReferencingLinks(t *testing.T) {
	old := BasePath
	BasePath = "/cairn"
	defer func() { BasePath = old }()

	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en, fr]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`rel="canonical"`, `property="og:url"`, `hreflang`} {
		if strings.Contains(string(m.Pages["en"].HTML), s) {
			t.Errorf("emitted %s with no site.url", s)
		}
	}
}

func ogLine(html string) string {
	for _, l := range strings.Split(html, "\n") {
		if strings.Contains(l, "og:image") {
			return strings.TrimSpace(l)
		}
	}
	return "(no og:image line)"
}
