package main

import (
	"strings"
	"testing"
)

func TestFaviconTaglineOGImageNoindex(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "url: https://tools.example.org\nlogo: /assets/logo.png\nfavicon: /assets/fav.png\n" +
			"index: false\ntagline: Small tools, kindly hosted\nlocales: [en]\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(cfg, nil)
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
