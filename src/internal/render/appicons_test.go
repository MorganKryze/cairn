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
			cfg:  &config.Config{Site: config.Site{Favicon: "/assets/brand.svg"}},
			want: []AppIcon{{Src: "/assets/brand.svg", Sizes: "any", Type: "image/svg+xml"}},
		},
		{
			name: "a raster we measured is declared at its real size",
			cfg: &config.Config{
				Site:        config.Site{Favicon: "/assets/brand.png"},
				FaviconDims: [2]int{512, 512},
			},
			want: []AppIcon{{Src: "/assets/brand.png", Sizes: "512x512", Type: "image/png"}},
		},
		{
			name: "a raster behind a URL carries no size: measuring it means a request we do not make",
			cfg:  &config.Config{Site: config.Site{Favicon: "https://cdn.example.org/brand.png"}},
			want: []AppIcon{{Src: "https://cdn.example.org/brand.png", Type: "image/png"}},
		},
		{
			name: "an unknown extension asserts neither size nor type",
			cfg:  &config.Config{Site: config.Site{Favicon: "https://cdn.example.org/brand"}},
			want: []AppIcon{{Src: "https://cdn.example.org/brand"}},
		},
		{
			name: "an explicit list wins outright: only the operator can know these",
			cfg: &config.Config{Site: config.Site{
				Favicon: "/assets/brand.png",
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
		Site: config.Site{Favicon: "https://cdn.example.org/brand"},
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
			got := AppIcons(&config.Config{Site: config.Site{Favicon: c.favicon}})
			if got[0].Src != c.want {
				t.Errorf("src = %q, want %q", got[0].Src, c.want)
			}
		})
	}
}
