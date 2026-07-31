// Package check backs the -check flag: it validates a config directory the
// way a boot would, then reports the warnings a serving site would hide.
package check

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	// og:image needs a url and a raster logo, both. With a url but no usable
	// logo, every link to the site previews as bare text on Mastodon, Slack or
	// anywhere else, and the page itself gives no hint of it.
	if cfg.Site.URL != "" && !cfg.Noindex() && !render.IsRaster(cfg.Site.Logo) {
		if cfg.Site.Logo == "" {
			out = append(out, "site.yaml has no logo: links to the site preview with no image (og:image wants a png, jpg, webp or gif)")
		} else {
			out = append(out, fmt.Sprintf("site.yaml logo %q is not a raster image: links to the site preview with no image (og:image wants a png, jpg, webp or gif)", cfg.Site.Logo))
		}
	}
	out = append(out, unresolvableRefs(cfg)...)
	out = append(out, categoryConfusion(cfg)...)
	out = append(out, inertSettings(cfg)...)
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
	// A file in media/ can be written two ways, and both are documented: the
	// bare name, and the /media/… path it is served at. Recording only the
	// first made -check announce a file as unreferenced while it sat on the
	// page, which is worse than saying nothing: it invites deleting it.
	markUsed := func(src string) {
		if rel, ok := strings.CutPrefix(src, "/media/"); ok {
			used[rel] = true
			return
		}
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

// unresolvableRefs finds paths that look like a file and reach nothing. cairn
// passes them through untouched, so a browser resolves them against whatever
// page it is on: `logo: logo.png` becomes /en/logo.png from a page and
// /logo.png from the manifest, and 404s from both. These stay warnings rather
// than errors because the damage is a missing image, and refusing to boot over
// one would replace a working site with the getting-started page.
func unresolvableRefs(cfg *config.Config) []string {
	var out []string
	looksLocal := func(v string) bool {
		return v != "" && !config.IsURLOrAbs(v) && !uriRe.MatchString(v)
	}
	if v := cfg.Site.Logo; looksLocal(v) {
		out = append(out, fmt.Sprintf("site.yaml logo %q resolves nowhere: it is neither a URL nor an /assets path, so the header image and og:image both point at a 404", v))
	}
	if v := cfg.Site.Favicon; looksLocal(v) {
		out = append(out, fmt.Sprintf("site.yaml favicon %q resolves nowhere: it is neither a URL nor an /assets path", v))
	}
	for _, set := range []struct {
		where string
		links []config.FooterLink
	}{{"links", cfg.Site.Links}, {"footer", cfg.Site.Footer}} {
		for _, l := range set.links {
			if looksLocal(l.URL) {
				out = append(out, fmt.Sprintf("site.yaml %s url %q resolves nowhere: a link needs a scheme (https://…, mailto:…) or an absolute path", set.where, l.URL))
			}
		}
	}
	return out
}

// categoryConfusion catches the two ways a category id goes wrong without a
// word from anyone: a service pointing at an id categories.yaml never defines,
// and two ids that differ only in case, which render as two sections carrying
// the same name.
func categoryConfusion(cfg *config.Config) []string {
	var out []string
	byLower := map[string][]string{}
	for _, c := range cfg.Categories {
		byLower[strings.ToLower(c.ID)] = append(byLower[strings.ToLower(c.ID)], c.ID)
	}
	lowers := make([]string, 0, len(byLower))
	for k := range byLower {
		lowers = append(lowers, k)
	}
	sort.Strings(lowers)
	for _, k := range lowers {
		if ids := byLower[k]; len(ids) > 1 {
			sort.Strings(ids)
			out = append(out, fmt.Sprintf("categories %s differ only by case, so the page shows that many sections under one name", strings.Join(quoteAll(ids), " and ")))
		}
	}
	return out
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

// A URI with a scheme: mailto: and tel: are legitimate link targets, so the
// "resolves nowhere" test must not flag them.
var uriRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

// inertSettings finds keys that were written, accepted, and then do nothing.
// Each one looks like a working configuration from the page: the operator gets
// no error, no log line, and no hint that the thing they asked for is off.
func inertSettings(cfg *config.Config) []string {
	var out []string

	// Every pill comes from Gatus. Without an address to poll, the companion
	// keys have nothing to act on and no pill is drawn at all.
	if st := cfg.Site.Status; st.Gatus == "" {
		var set []string
		if st.Page != "" {
			set = append(set, "status.page")
		}
		if st.Interval != "" {
			set = append(set, "status.interval")
		}
		if st.Linked != nil {
			set = append(set, "status.linked")
		}
		if len(set) > 0 {
			out = append(out, fmt.Sprintf("%s set without status.gatus: nothing polls, so no pill is drawn at all", strings.Join(set, " and ")))
		}
	}

	// Str falls through to the built-in table on a miss, so a misspelled key
	// is indistinguishable from no override.
	valid := map[string]bool{}
	for _, k := range config.StringKeys() {
		valid[k] = true
	}
	var unknown []string
	for k := range cfg.Site.Strings {
		if !valid[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		out = append(out, fmt.Sprintf("strings key %s is not one cairn uses, so the override does nothing (the keys are %s)",
			strings.Join(quoteAll(unknown), ", "), strings.Join(config.StringKeys(), ", ")))
	}

	// An icons list replaces cairn's whole set rather than adding to it, and
	// Chromium offers "install this site" only when a 192 and a 512 are both
	// declared. Adding one icon to improve the home screen silently removes
	// the install prompt.
	if len(cfg.Site.Icons) > 0 {
		has := func(size string) bool {
			for _, ic := range cfg.Site.Icons {
				for _, s := range strings.Fields(ic.Sizes) {
					if s == size || s == "any" {
						return true
					}
				}
			}
			return false
		}
		if !has("192x192") || !has("512x512") {
			out = append(out, "icons replaces cairn's whole set and this one has no 192x192 and 512x512 pair, so browsers stop offering to install the site")
		}
	}

	// The struct says icon is for header links; footer entries take it without
	// complaint and it is never rendered.
	for _, l := range cfg.Site.Footer {
		if l.Icon != "" {
			out = append(out, fmt.Sprintf("footer entry %q has an icon: only header links render one, so it is dropped", l.URL))
		}
	}
	return out
}
