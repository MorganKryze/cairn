package export

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// site is one config reaching every tree the server exposes: a hosted page, a
// detail page with a screenshot, an operator's icon and favicon, a font, a
// custom stylesheet, a security contact, and a monitor the export has to drop.
func site(t *testing.T) (cfgDir, assetsDir string) {
	t.Helper()
	cfgDir = testutil.WriteFiles(t, map[string]string{
		"site.yaml": `title: Tools
url: https://tools.example.org
locales: [en, fr]
favicon: /assets/fav.svg
theme:
  accent: "#247b7b"
  font: {family: Inter, file: inter.woff2}
security: {contact: "mailto:security@example.org"}
status: {gatus: https://status.example.org}
pages:
  - id: legal
    title: {en: Legal notice, fr: Mentions légales}
    body: Who runs this.
`,
		"services.yaml": `- id: pad
  url: https://pad.example.org
  name: Pad
  icon: /assets/icons/pad.svg
  details: Write together.
  images: [{src: shot.png}]
- id: wiki
  url: https://wiki.example.org
  name: Wiki
`,
		"custom.css": ":root{--x:1}\n",
	})
	assetsDir = t.TempDir()
	write(t, assetsDir, "fav.svg", "<svg/>")
	write(t, assetsDir, "icons/pad.svg", "<svg/>")
	write(t, assetsDir, ".git/config", "[remote] url = https://token@example.org/x.git")
	write(t, assetsDir, ".env", "SECRET=1")
	write(t, assetsDir, "icons/.hidden/x.svg", "<svg/>")
	write(t, assetsDir, "shared/logo.svg", "<svg/>")
	write(t, assetsDir, "icons/index.html", "<p>a page nobody reaches</p>")
	write(t, cfgDir, "media/shot.png", "png")
	write(t, cfgDir, "fonts/inter.woff2", "wOF2")
	return cfgDir, assetsDir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// read returns every file under dir, keyed by its slash path.
func read(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func export(t *testing.T, cfgDir, assetsDir string) map[string][]byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Run(cfgDir, assetsDir, out); err != nil {
		t.Fatal(err)
	}
	return read(t, out)
}

func exportSite(t *testing.T) map[string][]byte {
	t.Helper()
	cfgDir, assetsDir := site(t)
	return export(t, cfgDir, assetsDir)
}

var localRef = regexp.MustCompile(`(?:href|src)="(/[^"]*)"|url\("(/[^"]*)"\)`)

// A static host has nothing behind the files. Every address a page links has
// to be one of them, or a rule the host applies, or a visitor meets the 404.
func TestEveryLocalLinkLandsOnAnExportedFile(t *testing.T) {
	for _, base := range []string{"", "/tools"} {
		t.Run("base="+base, func(t *testing.T) {
			prev := render.BasePath
			render.BasePath = base
			t.Cleanup(func() { render.BasePath = prev })

			files := exportSite(t)
			redirects := string(files["_redirects"])
			checked := 0
			for name, b := range files {
				if !strings.HasSuffix(name, ".html") {
					continue
				}
				for _, m := range localRef.FindAllSubmatch(b, -1) {
					ref := string(m[1]) + string(m[2])
					ref, _, _ = strings.Cut(ref, "?")
					ref, _, _ = strings.Cut(ref, "#")
					rel, ok := strings.CutPrefix(ref, base+"/")
					if !ok {
						t.Errorf("%s links %s, outside the base path", name, ref)
						continue
					}
					if rel == "" || strings.HasSuffix(rel, "/") {
						rel += "index.html"
					}
					if _, ok := files[rel]; !ok && !strings.Contains(redirects, ref+" ") {
						t.Errorf("%s links %s, which the export does not carry", name, ref)
					}
					checked++
				}
			}
			if checked < 100 {
				t.Fatalf("only %d links checked: the pattern no longer matches the pages", checked)
			}
		})
	}
}

func TestTheExportCarriesTheWholeSite(t *testing.T) {
	files := exportSite(t)
	for _, want := range []string{
		"index.html", "404.html", "en/404.html", "fr/404.html",
		"en/index.html", "fr/index.html", "en/pad/index.html", "fr/legal/index.html",
		"robots.txt", "sitemap.xml", ".well-known/security.txt", "manifest.webmanifest", "custom.css",
		"assets/fav.svg", "assets/icons/pad.svg", "media/shot.png", "fonts/inter.woff2",
		"static/fonts/fraunces.woff2", "_headers", "_redirects", ".htaccess",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("%s is missing from the export", want)
		}
	}
	// favicon.ico redirects to the operator's icon on the server, so it is a
	// rule here rather than a copy of an svg under an .ico name.
	if _, ok := files["favicon.ico"]; ok {
		t.Error("favicon.ico was written as a file")
	}
	if got := string(files["_redirects"]); got != "/favicon.ico /assets/fav.svg 302\n" {
		t.Errorf("_redirects = %q", got)
	}
	if !bytes.Contains(files["404.html"], []byte("Page not found")) || !bytes.Contains(files["fr/404.html"], []byte("Page introuvable")) {
		t.Error("the 404 pages are not the localized ones")
	}
}

func TestNoDotfileLeavesTheAssetsTree(t *testing.T) {
	for name := range exportSite(t) {
		if strings.HasPrefix(name, "assets/") && (strings.Contains(name, "/.") || strings.Contains(name, "git")) {
			t.Errorf("%s was exported", name)
		}
	}
}

// Nothing polls on a static host. A pill would show whatever the monitor said
// the minute the export ran, for as long as the files stay up.
func TestTheExportShipsNoStatusPills(t *testing.T) {
	files := exportSite(t)
	for name, b := range files {
		if strings.HasSuffix(name, ".html") && (bytes.Contains(b, []byte("status-slot")) || bytes.Contains(b, []byte("/static/status."))) {
			t.Errorf("%s still carries a pill or status.js", name)
		}
	}
	if bytes.Contains(files["_headers"], []byte("connect-src")) || bytes.Contains(files[".htaccess"], []byte("connect-src")) {
		t.Error("the CSP still allows the fetch status.js makes")
	}
}

func TestTheHostRulesCarryTheServersHeaders(t *testing.T) {
	files := exportSite(t)
	for _, name := range []string{"_headers", ".htaccess"} {
		for _, want := range []string{"Content-Security-Policy", "X-Frame-Options", "Strict-Transport-Security", "public, max-age=31536000, immutable"} {
			if !bytes.Contains(files[name], []byte(want)) {
				t.Errorf("%s lacks %s", name, want)
			}
		}
	}
	for _, want := range []string{
		"ErrorDocument 404 /404.html\n",
		"<If \"%{REQUEST_URI} =~ m#^/fr/#\">\n  ErrorDocument 404 /fr/404.html\n</If>\n",
		"Redirect 302 /favicon.ico /assets/fav.svg\n",
	} {
		if !bytes.Contains(files[".htaccess"], []byte(want)) {
			t.Errorf(".htaccess lacks %q", want)
		}
	}
}

func TestTheExportNeedsTheSiteAddress(t *testing.T) {
	cfgDir := testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, name: Pad}\n",
	})
	out := filepath.Join(t.TempDir(), "site.zip")
	if _, err := Run(cfgDir, t.TempDir(), out); err == nil || !strings.Contains(err.Error(), "site.url") {
		t.Fatalf("err = %v, want one naming site.url", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a failed export left an archive behind")
	}
}

func TestADirectoryWithFilesInItIsRefused(t *testing.T) {
	cfgDir, assetsDir := site(t)
	out := t.TempDir()
	write(t, out, "keep.txt", "mine")
	if _, err := Run(cfgDir, assetsDir, out); err == nil {
		t.Fatal("the export wrote into a directory that already held files")
	}
	if b, _ := os.ReadFile(filepath.Join(out, "keep.txt")); string(b) != "mine" {
		t.Error("the refusal touched what was there")
	}
	empty := t.TempDir()
	if _, err := Run(cfgDir, assetsDir, empty); err != nil {
		t.Errorf("an empty directory was refused: %v", err)
	}
	if info, err := os.Stat(empty); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("the directory is not readable by a web server running as someone else: %v", info.Mode())
	}
}

// Each archive holds exactly what the directory export holds, and two runs of
// the same config on one day give the same bytes, so a deploy step can skip an
// upload that changes nothing.
func TestTheArchivesHoldTheSameFiles(t *testing.T) {
	cfgDir, assetsDir := site(t)
	want := export(t, cfgDir, assetsDir)
	for _, name := range []string{"site.zip", "site.tar", "site.tar.gz", "site.tgz"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "dist")
			first, second := filepath.Join(dir, "1-"+name), filepath.Join(dir, "2-"+name)
			for _, out := range []string{first, second} {
				if _, err := Run(cfgDir, assetsDir, out); err != nil {
					t.Fatal(err)
				}
			}
			a, _ := os.ReadFile(first)
			b, _ := os.ReadFile(second)
			if !bytes.Equal(a, b) {
				t.Error("two exports of the same config differ")
			}
			got := unpack(t, name, a)
			if len(got) != len(want) {
				t.Errorf("%d files, the directory export has %d", len(got), len(want))
			}
			for n, body := range want {
				if !bytes.Equal(got[n], body) {
					t.Errorf("%s differs from the directory export", n)
				}
			}
		})
	}
}

func unpack(t *testing.T, name string, b []byte) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if strings.HasSuffix(name, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			files[f.Name], _ = io.ReadAll(rc)
			rc.Close()
		}
		return files
	}
	var r io.Reader = bytes.NewReader(b)
	if !strings.HasSuffix(name, ".tar") {
		gz, err := gzip.NewReader(r)
		if err != nil {
			t.Fatal(err)
		}
		r = gz
	}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		files[h.Name], _ = io.ReadAll(tr)
	}
	return files
}

// The root page is the one address the server answers with a redirect. The
// export answers it with a page that makes the same choice in the browser, and
// lands on the first locale without a script.
func TestTheRootPageSendsEveryoneToALanguage(t *testing.T) {
	prev := render.BasePath
	render.BasePath = "/tools"
	t.Cleanup(func() { render.BasePath = prev })

	root := string(exportSite(t)["index.html"])
	for _, want := range []string{
		`data-locales="en fr"`,
		`data-base="/tools"`,
		`<noscript><meta http-equiv="refresh" content="0; url=/tools/en/"></noscript>`,
		`<a href="/tools/fr/" hreflang="fr">FR</a>`,
		`<link rel="alternate" hreflang="fr" href="https://tools.example.org/tools/fr/">`,
	} {
		if !strings.Contains(root, want) {
			t.Errorf("the root page lacks %s", want)
		}
	}
	if !regexp.MustCompile(`<script src="/tools/static/lang\.[0-9a-f]{8}\.js"></script>`).MatchString(root) {
		t.Error("the root page does not load lang.js under its stamped name")
	}
}

func TestTheOutputOrderIsStable(t *testing.T) {
	cfgDir, assetsDir := site(t)
	out := filepath.Join(t.TempDir(), "site.zip")
	if _, err := Run(cfgDir, assetsDir, out); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if err := zr.Close(); err != nil {
		t.Fatal(err)
	}
	if names[0] != "index.html" {
		t.Errorf("first entry %s, want index.html", names[0])
	}
	pages := []string{}
	for _, n := range names {
		if strings.HasSuffix(n, "/index.html") {
			pages = append(pages, n)
		}
	}
	if !sort.StringsAreSorted(pages) {
		t.Errorf("pages are not written in order: %v", pages)
	}
}

// The server follows a symlinked folder and serves what is behind it, and the
// export has to carry the same files. A link back up the tree must not loop.
func TestASymlinkedFolderIsFollowedOnce(t *testing.T) {
	cfgDir, assetsDir := site(t)
	if err := os.Symlink(filepath.Join(assetsDir, "shared"), filepath.Join(assetsDir, "icons", "shared")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := os.Symlink(assetsDir, filepath.Join(assetsDir, "shared", "loop")); err != nil {
		t.Fatal(err)
	}
	files := export(t, cfgDir, assetsDir)
	for _, want := range []string{"assets/shared/logo.svg", "assets/icons/shared/logo.svg"} {
		if _, ok := files[want]; !ok {
			t.Errorf("%s is missing", want)
		}
	}
	// The server answers /assets/icons/index.html with a redirect to a folder
	// it will not list, so nobody can reach it there, and it stays out.
	if _, ok := files["assets/icons/index.html"]; ok {
		t.Error("an index.html the server never serves was exported")
	}
}
