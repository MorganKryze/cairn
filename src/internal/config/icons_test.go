package config

import (
	"os"
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
