package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// The whole point of this list is that nothing in it is guessed. Every field
// is either measured, declared by the operator, or absent; the old code put
// "180x180" on whatever it was handed, which was a guess published as a fact.
func TestAppIconsNeverInventASize(t *testing.T) {
	for _, c := range []struct {
		name string
		cfg  *config.Config
		want []AppIcon
	}{
		{
			name: "no favicon at all: cairn's own set, including the two Chromium insists on",
			cfg:  &config.Config{},
			want: []AppIcon{
				{Src: "/static/touch-icon.png", Sizes: "180x180", Type: "image/png"},
				{Src: "/static/icon-192.png", Sizes: "192x192", Type: "image/png", Purpose: "any maskable"},
				{Src: "/static/icon-512.png", Sizes: "512x512", Type: "image/png", Purpose: "any maskable"},
			},
		},
		{
			name: "an svg favicon is scalable, so it serves every size the way ours does",
			cfg:  &config.Config{Site: config.Site{Favicon: config.ThemedRef{Light: "/assets/brand.svg"}}},
			want: []AppIcon{{Src: "/assets/brand.svg", Sizes: "any", Type: "image/svg+xml"}},
		},
		{
			name: "a raster we measured is declared at its real size",
			cfg: &config.Config{
				Site:        config.Site{Favicon: config.ThemedRef{Light: "/assets/brand.png"}},
				FaviconDims: [2]int{512, 512},
			},
			want: []AppIcon{{Src: "/assets/brand.png", Sizes: "512x512", Type: "image/png"}},
		},
		{
			name: "a raster behind a URL carries no size: measuring it means a request we do not make",
			cfg:  &config.Config{Site: config.Site{Favicon: config.ThemedRef{Light: "https://cdn.example.org/brand.png"}}},
			want: []AppIcon{{Src: "https://cdn.example.org/brand.png", Type: "image/png"}},
		},
		{
			name: "an unknown extension asserts neither size nor type",
			cfg:  &config.Config{Site: config.Site{Favicon: config.ThemedRef{Light: "https://cdn.example.org/brand"}}},
			want: []AppIcon{{Src: "https://cdn.example.org/brand"}},
		},
		{
			name: "an explicit list wins outright: only the operator can know these",
			cfg: &config.Config{Site: config.Site{
				Favicon: config.ThemedRef{Light: "/assets/brand.png"},
				Icons: []config.SiteIcon{
					{Src: "/assets/b-192.png", Sizes: "192x192"},
					{Src: "/assets/b-512.png", Sizes: "512x512", Purpose: "any maskable"},
				},
			}, FaviconDims: [2]int{64, 64}},
			want: []AppIcon{
				{Src: "/assets/b-192.png", Sizes: "192x192", Type: "image/png"},
				{Src: "/assets/b-512.png", Sizes: "512x512", Type: "image/png", Purpose: "any maskable"},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := AppIcons(c.cfg)
			if len(got) != len(c.want) {
				t.Fatalf("got %d icons, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("icon %d = %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// An absent field has to be absent from the json too, not present and empty:
// `"sizes": ""` is a malformed manifest entry, not a silent one.
func TestUnknownFieldsAreOmittedFromTheJSON(t *testing.T) {
	b, err := json.Marshal(AppIcons(&config.Config{
		Site: config.Site{Favicon: config.ThemedRef{Light: "https://cdn.example.org/brand"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"sizes", "type", "purpose"} {
		if strings.Contains(string(b), absent) {
			t.Errorf("%s appears in %s, want it omitted when unknown", absent, b)
		}
	}
	if !strings.Contains(string(b), "src") {
		t.Errorf("no src in %s", b)
	}
}

// A base path prefixes cairn's own icons, and equally the operator's /assets
// ones, or a site under /cairn would point its manifest at the domain root.
func TestAppIconsFollowTheBasePath(t *testing.T) {
	old := BasePath
	BasePath = "/cairn"
	defer func() { BasePath = old }()

	for _, c := range []struct{ name, favicon, want string }{
		{"cairn's own", "", "/cairn/static/touch-icon.png"},
		{"the operator's", "/assets/brand.svg", "/cairn/assets/brand.svg"},
		{"a remote one is left alone", "https://cdn.example.org/b.png", "https://cdn.example.org/b.png"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := AppIcons(&config.Config{Site: config.Site{Favicon: config.ThemedRef{Light: c.favicon}}})
			if got[0].Src != c.want {
				t.Errorf("src = %q, want %q", got[0].Src, c.want)
			}
		})
	}
}

// A leading slash alone used to mean "local", so a protocol-relative logo or
// icon was prefixed like one of our own routes and came out pointing nowhere:
//
//	AppURL("//cdn.example.org/logo.png") -> "/cairn//cdn.example.org/logo.png"
//	ogImage(site, "//cdn.example.org/l.png") -> "https://site//cdn.example.org/l.png"
//
// Both now pass through, because the value already names another origin.
func TestProtocolRelativeURLsAreNotOurs(t *testing.T) {
	old := BasePath
	BasePath = "/cairn"
	defer func() { BasePath = old }()

	for _, c := range []struct{ name, in, want string }{
		{"our own path is prefixed", "/static/x.png", "/cairn/static/x.png"},
		{"so is the operator's", "/assets/x.png", "/cairn/assets/x.png"},
		{"a protocol-relative URL is left alone", "//cdn.example.org/x.png", "//cdn.example.org/x.png"},
		{"and so is the backslash form", `/\cdn.example.org/x.png`, `/\cdn.example.org/x.png`},
		{"an absolute URL too", "https://cdn.example.org/x.png", "https://cdn.example.org/x.png"},
		{"a bare name is not ours to prefix", "x.png", "x.png"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := AppURL(c.in); got != c.want {
				t.Errorf("AppURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}

	const site = "https://tools.example.org"
	for _, c := range []struct{ in, want string }{
		{"/assets/logo.png", site + "/assets/logo.png"},              // made absolute
		{"//cdn.example.org/logo.png", "//cdn.example.org/logo.png"}, // already absolute
		{"https://cdn.example.org/l.png", "https://cdn.example.org/l.png"},
	} {
		if got := ogImage(site, c.in); got != c.want {
			t.Errorf("ogImage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// iOS never reads the manifest for the home screen: it reads the
// apple-touch-icon link alone. So an operator's own icons list has to reach
// that link too, or their icons would take effect on Android while an iPhone
// showed cairn's mark, which is the one outcome supplying your own prevents.
func TestTouchIconHonoursTheOperatorsList(t *testing.T) {
	for _, c := range []struct {
		name string
		site config.Site
		want string
	}{
		{"nothing set: cairn's own", config.Site{}, "/static/touch-icon.png"},
		{"an svg favicon cannot serve as one", config.Site{Favicon: config.ThemedRef{Light: "/assets/b.svg"}}, "/static/touch-icon.png"},
		{"a png favicon can", config.Site{Favicon: config.ThemedRef{Light: "/assets/b.png"}}, "/assets/b.png"},
		{
			"a list wins, largest first",
			config.Site{Favicon: config.ThemedRef{Light: "/assets/b.svg"}, Icons: []config.SiteIcon{
				{Src: "/assets/b-192.png", Sizes: "192x192"},
				{Src: "/assets/b-512.png", Sizes: "512x512"},
				{Src: "/assets/b-64.png", Sizes: "64x64"},
			}},
			"/assets/b-512.png",
		},
		{
			"whatever order it is written in",
			config.Site{Icons: []config.SiteIcon{
				{Src: "/assets/b-512.png", Sizes: "512x512"},
				{Src: "/assets/b-192.png", Sizes: "192x192"},
			}},
			"/assets/b-512.png",
		},
		{
			"a multi-size entry counts at its largest",
			config.Site{Icons: []config.SiteIcon{
				{Src: "/assets/b-256.png", Sizes: "256x256"},
				{Src: "/assets/b-ico.png", Sizes: "48x48 96x96 512x512"},
			}},
			"/assets/b-ico.png",
		},
		{
			// "any" is an svg, which iOS may not render; theirs still beats ours,
			// since a home screen falling back to a screenshot is neutral where
			// cairn's mark is wrong.
			"a list of svgs still wins",
			config.Site{Icons: []config.SiteIcon{{Src: "/assets/b.svg", Sizes: "any"}}},
			"/assets/b.svg",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := TouchIcon(&config.Config{Site: c.site}); got != c.want {
				t.Errorf("TouchIcon = %q, want %q", got, c.want)
			}
		})
	}
}

// End to end: the link in the head, not just the helper.
func TestTheHeadCarriesTheOperatorsTouchIcon(t *testing.T) {
	cfg := &config.Config{Site: config.Site{
		Locales: []string{"en"},
		Icons:   []config.SiteIcon{{Src: "/assets/brand-512.png", Sizes: "512x512"}},
	}}
	cfg.Site.Theme.Accent = "#247b7b"
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	html := string(m.Pages["en"].HTML)
	if !strings.Contains(html, `<link rel="apple-touch-icon" href="/assets/brand-512.png">`) {
		t.Error("the head does not point at the operator's icon")
	}
	if strings.Contains(html, "touch-icon.png") {
		t.Error("cairn's own touch icon is still in the head")
	}
}
