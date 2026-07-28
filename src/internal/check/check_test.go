package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestCheckWarnings(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\nabout: {fr: Bonjour}\n",
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: {fr: Outil, en: Tool}\n  desc: {fr: Une phrase}\n" +
			"- id: b\n  url: https://b.example.org\n  name: Partout\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "orphan.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")
	for _, want := range []string{
		"site about has no en",
		`service "a" desc has no en`,
		"media/orphan.png is not referenced",
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	if strings.Contains(warnings, `"b"`) {
		t.Errorf("plain-string service should not warn:\n%s", warnings)
	}
}

func TestCheckMarkdownImageCountsAsUsed(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\nabout: \"See ![plan](plan.png)\"\n",
		"services.yaml": "- {id: a, url: https://a.example.org, name: A}\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "plan.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := checkWarnings(cfg, dir); len(w) != 0 {
		t.Errorf("warnings = %v, want none", w)
	}
}

// -check is what an operator wires into their own pipeline, so its warnings
// have to name the thing that is wrong and stay quiet about the rest.
func TestWarningsNameWhatIsMissing(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr, en]\n" +
			"tagline: {fr: Seulement en français}\n" +
			"about: {fr: Bonjour, en: Hello}\n" +
			"pages: [{id: legal, title: {fr: Mentions}, body: {fr: Texte, en: Text}, " +
			"sections: [{title: {fr: Éditeur}, body: {fr: Nom, en: Name}}]}]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: {fr: Bloc, en: Pad}, desc: {fr: Écrire}}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(checkWarnings(cfg, dir), "\n")

	// each field covering some locales but not all is named
	for _, want := range []string{"tagline", "legal", "pad", "section 1 title"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings never mention %q:\n%s", want, warnings)
		}
	}
	// a field translated everywhere is not mentioned
	if strings.Contains(warnings, "about") {
		t.Errorf("a fully translated field was flagged:\n%s", warnings)
	}
}

// A config with nothing missing says so and warns about nothing, which is what
// makes the warnings worth reading when they do appear.
func TestCompleteConfigWarnsAboutNothing(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad, desc: Write together.}\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := checkWarnings(cfg, dir); len(w) != 0 {
		t.Errorf("a complete config produced warnings: %v", w)
	}
	if code := RunCheck(dir); code != 0 {
		t.Errorf("RunCheck = %d, want 0", code)
	}
}

// A media folder is reported when a file is there that nothing references, and
// when a referenced image is heavy enough to hurt a phone.
func TestMediaWarnings(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	media := filepath.Join(dir, "media")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "orphan.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(mediaWarnings(cfg, media), "\n")
	if !strings.Contains(warnings, "orphan.png") {
		t.Errorf("an unreferenced media file was not reported:\n%s", warnings)
	}
}

// The exit code is the contract: anything that would not boot has to be a 1,
// whether the YAML is invalid or the directory is not there at all.
func TestRunCheckRefusals(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
	}{
		{"a service with no url", map[string]string{"services.yaml": "- {id: pad, name: Pad}\n"}},
		{"a site that is not a mapping", map[string]string{
			"site.yaml":     "- just a list\n",
			"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n"}},
		{"no services file at all", map[string]string{"site.yaml": "locales: [en]\n"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if code := RunCheck(testutil.WriteFiles(t, c.files)); code != 1 {
				t.Errorf("RunCheck = %d, want 1", code)
			}
		})
	}
	if code := RunCheck(filepath.Join(t.TempDir(), "absent")); code != 1 {
		t.Errorf("RunCheck on a missing directory = %d, want 1", code)
	}
}
