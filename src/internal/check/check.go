// Package check backs the -check flag: it validates a config directory the
// way a boot would, then reports the warnings a serving site would hide.
package check

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
)

func RunCheck(dir string) int {
	cfg, err := config.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := render.BuildModel(cfg, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, w := range checkWarnings(cfg, dir) {
		fmt.Println("warning:", w)
	}
	fmt.Printf("ok: %d services, %d categories, %d pages, locales %v\n",
		config.CountServices(cfg), len(cfg.Categories), len(cfg.Site.Pages), cfg.Site.Locales)
	return 0
}

func checkWarnings(cfg *config.Config, dir string) []string {
	var out []string
	out = append(out, missingTranslations(cfg)...)
	out = append(out, mediaWarnings(cfg, filepath.Join(dir, "media"))...)
	// The canonical, og:url and hreflang links are all built from site.url, so
	// without it they are simply absent. Nothing fails, the page looks right,
	// and a multilingual site quietly reads as several duplicates. Skipped
	// when the operator has already told search engines to stay away.
	if cfg.Site.URL == "" && !cfg.Noindex() {
		out = append(out, "site.yaml has no url: pages carry no canonical link, no og:url and no hreflang alternates (set url: https://… to emit them)")
	}
	if slugs := config.CdnSlugs(cfg); len(slugs) > 0 {
		out = append(out, fmt.Sprintf("%d icons load from a CDN in visitors' browsers (%s); run cairn -emit-icons to self-host them", len(slugs), strings.Join(slugs, ", ")))
	}
	return out
}

// missingTranslations lists every translatable field that covers some of
// the enabled locales but not all of them. A plain string covers them all.
func missingTranslations(cfg *config.Config) []string {
	var out []string
	warn := func(where string, l config.LString) {
		if len(l) == 0 || l[""] != "" {
			return
		}
		var missing []string
		for _, loc := range cfg.Site.Locales {
			if l[loc] == "" {
				missing = append(missing, loc)
			}
		}
		if len(missing) > 0 {
			out = append(out, fmt.Sprintf("%s has no %s", where, strings.Join(missing, ", ")))
		}
	}
	warn("site tagline", cfg.Site.Tagline)
	warn("site about", cfg.Site.About)
	for _, p := range cfg.Site.Pages {
		warn(fmt.Sprintf("page %q title", p.ID), p.Title)
		warn(fmt.Sprintf("page %q body", p.ID), p.Body)
		for i, s := range p.Sections {
			warn(fmt.Sprintf("page %q section %d title", p.ID, i+1), s.Title)
			warn(fmt.Sprintf("page %q section %d body", p.ID, i+1), s.Body)
		}
	}
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			warn(fmt.Sprintf("service %q name", s.ID), s.Name)
			warn(fmt.Sprintf("service %q desc", s.ID), s.Desc)
			warn(fmt.Sprintf("service %q details", s.ID), s.Details)
			for _, img := range s.Images {
				warn(fmt.Sprintf("service %q image caption", s.ID), img.Caption)
			}
		}
	}
	return out
}

var mdImgRefRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

// mediaWarnings flags files nothing references and files heavy enough to
// hurt the visitors of the pages that show them.
func mediaWarnings(cfg *config.Config, mediaDir string) []string {
	used := map[string]bool{}
	// Only a bare name refers to a file in media/; anything else is a link.
	markUsed := func(src string) {
		if !config.IsURLOrAbs(src) {
			used[src] = true
		}
	}
	texts := []config.LString{cfg.Site.About}
	for _, p := range cfg.Site.Pages {
		texts = append(texts, p.Body)
		for _, s := range p.Sections {
			texts = append(texts, s.Body)
		}
	}
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			texts = append(texts, s.Details)
			for _, img := range s.Images {
				markUsed(img.Src)
			}
		}
	}
	for _, l := range texts {
		for _, text := range l {
			for _, m := range mdImgRefRe.FindAllStringSubmatch(text, -1) {
				markUsed(m[1])
			}
		}
	}

	var out []string
	_ = filepath.WalkDir(mediaDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(mediaDir, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !used[rel] {
			out = append(out, fmt.Sprintf("media/%s is not referenced by any service or page", rel))
		}
		if info, ierr := d.Info(); ierr == nil && info.Size() > 1<<20 {
			out = append(out, fmt.Sprintf("media/%s weighs %.1f MB; visitors download it in full, consider a smaller export", rel, float64(info.Size())/(1<<20)))
		}
		return nil
	})
	return out
}
