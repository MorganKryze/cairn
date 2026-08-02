package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A logo, a favicon and a service icon are one image for most sites and two
// for the ones whose artwork is monochrome, so the field reads as either. The
// plain string is what every config written so far says, and it has to keep
// meaning exactly what it meant: one image, both themes.
func TestAThemedRefReadsAStringOrThePairOfThemes(t *testing.T) {
	for _, c := range []struct {
		name        string
		yaml        string
		light, dark string
	}{
		{"a plain string is the image for both themes", "/assets/logo.svg", "/assets/logo.svg", ""},
		{"a slug is a string like any other", "hedgedoc", "hedgedoc", ""},
		{"the pair names the theme each one appears in", "{light: a.svg, dark: b.svg}", "a.svg", "b.svg"},
		{"written across lines", "light: a.svg\ndark: b.svg\n", "a.svg", "b.svg"},
		{"an absent field stays empty", "", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			var got ThemedRef
			if err := yaml.Unmarshal([]byte(c.yaml), &got); err != nil {
				t.Fatalf("refused %q: %v", c.yaml, err)
			}
			if got.Light != c.light || got.Dark != c.dark {
				t.Errorf("%q decoded to {light:%q dark:%q}, want {light:%q dark:%q}", c.yaml, got.Light, got.Dark, c.light, c.dark)
			}
		})
	}
}

// Half a pair is the shape that would paint nothing in one theme and leave no
// trace of why, so it is refused at the file. The message has to teach the
// other key rather than name the one that is there.
func TestAThemedRefRefusesHalfAPair(t *testing.T) {
	for _, c := range []struct{ name, yaml string }{
		{"only the light one", "{light: a.svg}"},
		{"only the dark one", "{dark: b.svg}"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var got ThemedRef
			err := yaml.Unmarshal([]byte(c.yaml), &got)
			if err == nil {
				t.Fatalf("accepted %q, which paints nothing in one theme", c.yaml)
			}
			for _, want := range []string{"light", "dark"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message does not teach %q: %v", want, err)
				}
			}
		})
	}
}

// The trap this shape invites: writing the theme names as anything else, or
// reaching for the suffix convention the icon collections use, where -light
// names the ink rather than the theme.
func TestAThemedRefNamesTheKeyThatIsWrong(t *testing.T) {
	var got ThemedRef
	err := yaml.Unmarshal([]byte("{light: a.svg, on_dark: b.svg}"), &got)
	if err == nil {
		t.Fatal("accepted a key that is neither light nor dark")
	}
	if !strings.Contains(err.Error(), "on_dark") {
		t.Errorf("message does not name the key that is wrong: %v", err)
	}
}

// Themed reports whether the two themes differ, which is what decides between
// plain markup and a rule in the page's stylesheet.
func TestThemedIsOnlyTrueForATruePair(t *testing.T) {
	for _, c := range []struct {
		ref  ThemedRef
		want bool
	}{
		{ThemedRef{}, false},
		{ThemedRef{Light: "a.svg"}, false},
		{ThemedRef{Light: "a.svg", Dark: "b.svg"}, true},
		{ThemedRef{Light: "a.svg", Dark: "a.svg"}, false},
	} {
		if got := c.ref.Themed(); got != c.want {
			t.Errorf("%+v.Themed() = %v, want %v", c.ref, got, c.want)
		}
	}
}

// The dark half lands in the page's stylesheet, inside url(), which is a
// context nothing escapes for us. It is the trap theme.font.file fell into:
// a value can pass every check about where it points and still carry the one
// sequence that ends a style element.
func TestADarkImageCannotEndTheStyleElement(t *testing.T) {
	for _, c := range []struct{ name, val string }{
		{"a logo closing the style element", "/assets/a</style><script>alert(1)</script>b.svg"},
		{"a logo opening a tag", "/assets/<img src=x>.svg"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Site{Locales: []string{"en"}}
			s.Theme.Accent = "#247b7b"
			s.Logo = ThemedRef{Light: "/assets/logo.svg", Dark: c.val}
			err := validateSite(s, map[string]string{})
			if err == nil {
				t.Fatalf("accepted %q, which reaches the page as markup", c.val)
			}
			for _, want := range []string{"logo.dark", "stylesheet"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message missing %q: %v", want, err)
				}
			}
		})
	}
}

// And the ordinary shapes keep loading: the guard is about two characters, not
// about narrowing what an image URL may be.
func TestTheDarkGuardLeavesOrdinaryImagesAlone(t *testing.T) {
	for _, val := range []string{
		"/assets/logo-white.svg",
		"https://cdn.example.org/logo~white (1).png?v=2&x=1",
		"/assets/dossier accentué/logo.svg",
	} {
		s := &Site{Locales: []string{"en"}}
		s.Theme.Accent = "#247b7b"
		s.Logo = ThemedRef{Light: "/assets/logo.svg", Dark: val}
		if err := validateSite(s, map[string]string{}); err != nil {
			t.Errorf("refused an ordinary image %q: %v", val, err)
		}
	}
}

// The same rule on a service icon, which reaches the same stylesheet.
func TestADarkServiceIconCannotEndTheStyleElement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "services.yaml"), []byte(
		"- {id: a, url: https://a.example.org, name: A, icon: {light: github, dark: \"/assets/x</style><script>y</script>.svg\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("accepted a dark icon that ends the style element")
	}
	for _, want := range []string{"icon.dark", "stylesheet"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message missing %q: %v", want, err)
		}
	}
}
