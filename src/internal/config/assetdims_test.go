package config

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// A 2x1 png: the smallest image whose width and height differ, so a test
// cannot pass by happening to return a square.
func png2x1(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The manifest states the favicon's size only when it can measure it, so this
// is the line between a fact and a guess. Everything it cannot open has to
// come back zero rather than plausible.
func TestAssetDimsMeasuresOnlyWhatItCanOpen(t *testing.T) {
	assets := t.TempDir()
	if err := os.WriteFile(filepath.Join(assets, "brand.png"), png2x1(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "brand.svg"), []byte(`<svg viewBox="0 0 9 9"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(assets, "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "deep", "nested.png"), png2x1(t), 0o600); err != nil {
		t.Fatal(err)
	}

	old := AssetsPath
	AssetsPath = assets
	defer func() { AssetsPath = old }()

	for _, c := range []struct {
		name, ref string
		want      [2]int
	}{
		{"a raster in the mount is measured", "/assets/brand.png", [2]int{2, 1}},
		{"and so is one in a subfolder", "/assets/deep/nested.png", [2]int{2, 1}},
		{"an svg has no intrinsic size to report", "/assets/brand.svg", [2]int{}},
		{"a remote file would need a request we do not make", "https://cdn.example.org/b.png", [2]int{}},
		{"a path outside the mount is not ours to read", "/static/touch-icon.png", [2]int{}},
		{"nor is one that climbs out of it", "/assets/../../etc/hosts", [2]int{}},
		{"a file that is not there is simply unknown", "/assets/absent.png", [2]int{}},
		{"the mount root itself is not a file", "/assets/", [2]int{}},
		{"no favicon at all", "", [2]int{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := assetDims(c.ref); got != c.want {
				t.Errorf("assetDims(%q) = %v, want %v", c.ref, got, c.want)
			}
		})
	}
}

// Load has to carry the measurement through, or AppIcons has nothing to go on.
func TestLoadMeasuresTheFavicon(t *testing.T) {
	assets := t.TempDir()
	if err := os.WriteFile(filepath.Join(assets, "brand.png"), png2x1(t), 0o600); err != nil {
		t.Fatal(err)
	}
	old := AssetsPath
	AssetsPath = assets
	defer func() { AssetsPath = old }()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"),
		[]byte("locales: [en]\nfavicon: /assets/brand.png\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "services.yaml"),
		[]byte("- {id: pad, url: https://pad.example.org, name: Pad}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FaviconDims != [2]int{2, 1} {
		t.Errorf("FaviconDims = %v, want [2 1]", cfg.FaviconDims)
	}
}
