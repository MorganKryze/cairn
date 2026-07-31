package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// A slug resolves to the operator's own file when there is one, and falls
// back to the CDN otherwise; -emit-icons lists exactly the ones still remote.
func TestLocalIconResolution(t *testing.T) {
	assets := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assets, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "icons", "immich.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := AssetsPath
	AssetsPath = assets
	defer func() { AssetsPath = old }()

	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, icon: immich}\n" +
			"- {id: b, url: https://b.example.org, name: B, icon: nextcloud}\n",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := IconURL(cfg, "immich"); got != "/assets/icons/immich.svg" {
		t.Errorf("local icon = %q, want /assets/icons/immich.svg", got)
	}
	if got := IconURL(cfg, "nextcloud"); !strings.HasPrefix(got, iconCDN) {
		t.Errorf("missing local file should fall back to CDN, got %q", got)
	}
	script := string(EmitIconsScript(cfg))
	if strings.Contains(script, "immich") || !strings.Contains(script, "icons/nextcloud.svg") {
		t.Errorf("emit-icons should list only CDN-bound slugs:\n%s", script)
	}
}

// An icon slug becomes a path segment on the CDN and a filename in the script
// -emit-icons writes, which the docs tell the operator to pipe into sh. Load is
// the only place that can refuse a hostile one: by the time the script exists
// the slug is already text in a command line. Nothing outside this shape could
// ever resolve upstream anyway, so the refusal costs an operator nothing, and
// the message has to show the shape rather than only name the field.
func TestHostileIconSlugIsRefused(t *testing.T) {
	for _, c := range []struct{ name, icon string }{
		{"a command substitution", "$(touch pwned)"},
		{"a chained command", "immich; touch pwned"},
		{"a pipe into another shell", "immich | sh"},
		{"a backtick substitution", "`touch pwned`"},
		{"a quote that would close the one around it", `immich'; touch pwned; '`},
		{"a newline opening a second command", "immich\ntouch pwned"},
		{"a path climbing out of icons/", "../../etc/passwd"},
		{"a slash, which writes outside icons/", "homarr/immich"},
		{"a leading dash, which curl reads as a flag", "-o"},
		{"an uppercase name the CDN has no file for", "Immich"},
		{"a space, which splits the curl argument in two", "two words"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// %q gives a double-quoted scalar, which yaml reads the same way Go
			// does, so a comma or a newline in the value survives the flow map.
			doc := fmt.Sprintf("- {id: a, url: https://a.example.org, name: A, icon: %q}\n", c.icon)
			_, err := parseServices("services.yaml", []byte(doc))
			if err == nil {
				t.Fatalf("icon %q was accepted, and reaches the emitted script from there", c.icon)
			}
			for _, want := range []string{"is not a slug", "hedgedoc"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q does not mention %q", err, want)
				}
			}
		})
	}
}

// The other half of that rule. Every shape an operator actually writes has to
// keep loading, or the slug check would have quietly broken every config that
// names an icon at all.
func TestOrdinaryIconValuesStillLoad(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"services.yaml": "- {id: a, url: https://a.example.org, name: A, icon: hedgedoc}\n" +
			"- {id: b, url: https://b.example.org, name: B, icon: home-assistant}\n" +
			"- {id: c, url: https://c.example.org, name: C, icon: 2fauth}\n" +
			"- {id: d, url: https://d.example.org, name: D, icon: /assets/icons/own.svg}\n" +
			"- {id: e, url: https://e.example.org, name: E, icon: https://cdn.example.org/i.svg}\n" +
			"- {id: f, url: https://f.example.org, name: F}\n",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("a config naming ordinary icons was refused: %v", err)
	}
	if got := IconURL(cfg, "home-assistant"); got != iconCDN+"home-assistant.svg" {
		t.Errorf("IconURL(home-assistant) = %q, want the CDN file", got)
	}
	// Only the three slugs are CDN-bound: the /assets file, the full URL and the
	// service with no icon at all have nothing to download.
	if got := strings.Join(CdnSlugs(cfg), ","); got != "2fauth,hedgedoc,home-assistant" {
		t.Errorf("CdnSlugs = %q, want the three slugs alone", got)
	}
}

// shellQuote is the second line of defence behind the slug rule: single quotes
// are the one form sh treats as fully literal, and a quote inside the value has
// to close them, escape itself and reopen them. Wrapping without that hands the
// rest of the line back to the shell as commands.
func TestShellQuote(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"a plain value is quoted anyway", "icons/immich.svg", `'icons/immich.svg'`},
		{"a single quote closes, escapes and reopens", "it's", `'it'\''s'`},
		{"a value that is nothing but a quote", "'", `''\'''`},
		{"the shape that would end our quoting and start a command",
			"a'; touch pwned; '", `'a'\''; touch pwned; '\'''`},
		{"a space stays inside one word", "two words", `'two words'`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := shellQuote(c.in); got != c.want {
				t.Errorf("shellQuote(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

// And what the table above claims is what a real sh does with it: the value
// comes back byte for byte, with nothing run on the way. The command runs in a
// temporary directory, so a quoting that ever regresses leaves its evidence
// there rather than in the working tree.
func TestShellQuoteIsLiteralToSh(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh to check the quoting against")
	}
	for _, in := range []string{
		"icons/immich.svg", "it's", "'", `a'; touch pwned; '`, "two words", "$(touch pwned)", "`touch pwned`",
	} {
		cmd := exec.Command(sh, "-c", "printf %s "+shellQuote(in))
		cmd.Dir = t.TempDir()
		out, err := cmd.Output()
		if err != nil {
			t.Errorf("sh refused the quoting of %q: %v", in, err)
			continue
		}
		if string(out) != in {
			t.Errorf("sh read %q as %q, want it verbatim", in, string(out))
		}
	}
}
