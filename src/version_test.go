package main

import (
	"strings"
	"testing"
)

// The footer stamp reads differently depending on where the binary came from,
// and each shape has to point somewhere useful, or nowhere at all.
func TestVersionInfo(t *testing.T) {
	for _, c := range []struct{ in, label, href string }{
		{"1.8.0", "1.8.0", repoURL + "/releases/tag/v1.8.0"},
		{"1.8.0-rc1", "1.8.0-rc1", repoURL + "/releases/tag/v1.8.0-rc1"},
		{"10.2.13", "10.2.13", repoURL + "/releases/tag/v10.2.13"},
		{"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", "@a1b2c3d", repoURL + "/commit/a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"},
		{"a1b2c3d", "@a1b2c3d", repoURL + "/commit/a1b2c3d"},
		// nothing public to link to for these
		{"dev", "dev", ""},
		{"unstable", "unstable", ""},
		{"", "", ""},
	} {
		label, href := versionInfo(c.in)
		if label != c.label || href != c.href {
			t.Errorf("versionInfo(%q) = (%q, %q), want (%q, %q)", c.in, label, href, c.label, c.href)
		}
	}
}

// Off by default: an operator who never heard of the key sees the footer they
// already had.
func TestShowVersionIsOptIn(t *testing.T) {
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	if html := string(current.Load().Pages["en"].HTML); strings.Contains(html, "foot-version") {
		t.Error("the version stamp showed up without show_version")
	}
}

func TestShowVersionRenders(t *testing.T) {
	prev := version
	t.Cleanup(func() { version = prev })

	// a tagged build names its release and links to it
	version = "1.8.0"
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\nshow_version: true\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html := string(current.Load().Pages["en"].HTML)
	want := `<a class="foot-version" href="` + repoURL + `/releases/tag/v1.8.0">1.8.0</a>`
	if !strings.Contains(html, want) {
		t.Errorf("missing the release stamp %s", want)
	}

	// an untagged build names its commit instead
	version = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\nshow_version: true\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html = string(current.Load().Pages["en"].HTML)
	if !strings.Contains(html, `/commit/a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0">@a1b2c3d</a>`) {
		t.Error("missing the commit stamp")
	}

	// a local build is named but not linked
	version = "dev"
	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\nshow_version: true\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	html = string(current.Load().Pages["en"].HTML)
	if !strings.Contains(html, `<span class="foot-version">dev</span>`) {
		t.Error("a local build should be named without a link")
	}
}

// The stamp lives inside the credit, so turning the credit off takes it along:
// no orphan version floating in an otherwise empty footer.
func TestShowVersionFollowsTheCredit(t *testing.T) {
	prev := version
	version = "1.8.0"
	t.Cleanup(func() { version = prev })

	storeModel(t, map[string]string{
		"site.yaml":     "locales: [en]\nshow_version: true\ncredit: false\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	if html := string(current.Load().Pages["en"].HTML); strings.Contains(html, "foot-version") {
		t.Error("the version stamp outlived the credit it belongs to")
	}
}
