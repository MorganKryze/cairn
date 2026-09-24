// Package export writes a site out as files a static host serves as they are:
// every page the server answers, every file it serves, and the rules a host
// needs to send the headers and the 404 page cairn would.
//
// Every byte comes out of the server's own handler, asked in process, so an
// exported page cannot drift from the page the server renders.
package export

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/server"
)

// optional are the addresses a site answers or not depending on its config:
// security.txt needs a contact, the sitemap goes with noindex, custom.css
// exists when the operator wrote one, and favicon.ico redirects to an
// operator's own icon.
var optional = []string{
	"/robots.txt",
	"/sitemap.xml",
	"/.well-known/security.txt",
	"/manifest.webmanifest",
	"/favicon.ico",
	"/custom.css",
}

type redirect struct{ from, to string }

type exporter struct {
	h         http.Handler
	site      *url.URL
	out       sink
	redirects []redirect
	// immutable lists the stamped assets, and forever is the Cache-Control the
	// server gave them, so a host caches them as long as cairn would.
	immutable []string
	forever   string
	files     int
}

// Run loads cfgDir, renders it, and writes the result to out: a directory, or
// an archive when out ends in .zip, .tar, .tar.gz or .tgz. It returns the
// number of files written.
func Run(cfgDir, assetsDir, out string) (int, error) {
	cfg, err := config.Load(cfgDir)
	if err != nil {
		return 0, err
	}
	if cfg.Site.URL == "" {
		return 0, errors.New("export: site.url is empty; the sitemap, robots.txt and the canonical links need the address the site will be served at")
	}
	site, err := url.Parse(cfg.Site.URL)
	if err != nil {
		return 0, fmt.Errorf("export: site.url: %w", err)
	}
	if addr := cfg.Site.StatusAddress(); addr != "" {
		log.Printf("export: status pills need a server to poll %s; the exported pages carry none", addr)
		var none config.Site
		cfg.Site.Status = none.Status
	}
	m, err := render.BuildModel(cfg, nil)
	if err != nil {
		return 0, err
	}
	root, err := render.RootPage(cfg)
	if err != nil {
		return 0, err
	}
	server.Store(m)

	s, commit, err := open(out)
	if err != nil {
		return 0, err
	}
	e := &exporter{h: server.Handler(cfgDir, assetsDir), site: site, out: s}
	if err := e.write(m, root, cfgDir, assetsDir); err != nil {
		return 0, errors.Join(err, s.abort())
	}
	return e.files, commit()
}

func (e *exporter) write(m *render.Model, root []byte, cfgDir, assetsDir string) error {
	if err := e.add("index.html", root); err != nil {
		return err
	}
	keys := make([]string, 0, len(m.Pages))
	for k := range m.Pages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := e.fetch("/"+k+"/", k+"/index.html"); err != nil {
			return err
		}
	}
	// The root one is what a host with a single error page serves; the others
	// are for a host that looks for the nearest 404.html up the tree.
	def := m.Cfg.DefaultLocale()
	if err := e.add("404.html", m.NotFound[def].HTML); err != nil {
		return err
	}
	for _, loc := range m.Cfg.Site.Locales {
		if err := e.add(loc+"/404.html", m.NotFound[loc].HTML); err != nil {
			return err
		}
	}
	for _, p := range optional {
		if err := e.fetchOptional(p); err != nil {
			return err
		}
	}
	if err := e.static(); err != nil {
		return err
	}
	for _, t := range []struct{ dir, prefix string }{
		{assetsDir, "assets"},
		{filepath.Join(cfgDir, "media"), "media"},
		{filepath.Join(cfgDir, "fonts"), "fonts"},
	} {
		if err := e.tree(t.dir, t.prefix); err != nil {
			return err
		}
	}
	return e.rules(m.Cfg)
}

func (e *exporter) add(name string, b []byte) error {
	e.files++
	return e.out.add(name, b)
}

// get asks the handler for path the way a visitor at site.url would, so the
// scheme decides HSTS and the cookies exactly as it does behind a proxy.
func (e *exporter) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, (&url.URL{Path: render.BasePath + path}).RequestURI(), nil)
	req.Host = e.site.Host
	req.Header.Set("X-Forwarded-Proto", e.site.Scheme)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *exporter) fetch(path, name string) error {
	rec := e.get(path)
	if rec.Code != http.StatusOK {
		return fmt.Errorf("export: %s answered %d", path, rec.Code)
	}
	return e.add(name, rec.Body.Bytes())
}

func (e *exporter) fetchOptional(path string) error {
	rec := e.get(path)
	switch rec.Code {
	case http.StatusOK:
		return e.add(strings.TrimPrefix(path, "/"), rec.Body.Bytes())
	case http.StatusNotFound:
		return nil
	case http.StatusFound, http.StatusMovedPermanently:
		e.redirects = append(e.redirects, redirect{render.BasePath + path, rec.Header().Get("Location")})
		return nil
	}
	return fmt.Errorf("export: %s answered %d", path, rec.Code)
}

// static writes cairn's own files under the names the pages link: stamped,
// except the fonts the stylesheet reaches by their plain name.
func (e *exporter) static() error {
	return fs.WalkDir(render.Embedded, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		path := strings.TrimPrefix(render.AssetURL(strings.TrimPrefix(p, "assets/")), render.BasePath)
		rec := e.get(path)
		if rec.Code != http.StatusOK {
			return fmt.Errorf("export: %s answered %d", path, rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
			e.immutable = append(e.immutable, filepath.Base(path))
			e.forever = cc
		}
		return e.add(strings.TrimPrefix(path, "/"), rec.Body.Bytes())
	})
}

// tree mirrors one of the directories the server exposes, and exports what
// the server would answer for each file in it. A dotfile never leaves: the
// server refuses to serve one, and an assets directory pointed at a working
// copy would otherwise publish .git/config in the archive.
func (e *exporter) tree(dir, prefix string) error {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return e.walk(dir, prefix, map[string]bool{})
}

// walk follows a symlinked directory, as the server's file handler does, and
// seen stops a link that points back up the tree from walking it forever.
func (e *exporter) walk(dir, prefix string, seen map[string]bool) error {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if seen[real] {
		return nil
	}
	seen[real] = true
	return filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != real && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(real, p)
		if err != nil {
			return err
		}
		name := prefix + "/" + filepath.ToSlash(rel)
		if d.Type()&fs.ModeSymlink != 0 {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				return e.walk(p, name, seen)
			}
		}
		if d.IsDir() {
			return nil
		}
		rec := e.get("/" + name)
		switch rec.Code {
		case http.StatusOK:
			return e.add(name, rec.Body.Bytes())
		// An index.html redirects to its folder, which the server refuses to
		// list, and a broken link has nothing behind it.
		case http.StatusMovedPermanently, http.StatusNotFound:
			log.Printf("export: %s left out; the server does not serve it either", name)
			return nil
		}
		return fmt.Errorf("export: /%s answered %d", name, rec.Code)
	})
}
