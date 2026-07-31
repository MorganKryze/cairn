// Package check backs the -check flag: it validates a config directory the
// way a boot would, then reports the warnings a serving site would hide.
package check

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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
	out = append(out, partialStringOverrides(cfg)...)
	out = append(out, unsupportedLocales(cfg)...)
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
	//
	// index: false does not silence this the way it silences the canonical
	// links above. og:image feeds chat unfurls, not crawlers, and a portal
	// kept out of search results is precisely the one that gets pasted into a
	// team channel, where a preview is all anyone sees before clicking.
	if cfg.Site.URL != "" && !render.IsRaster(cfg.Site.Logo) {
		if cfg.Site.Logo == "" {
			out = append(out, "site.yaml has no logo: links to the site preview with no image (og:image wants a png, jpg, webp or gif)")
		} else {
			out = append(out, fmt.Sprintf("site.yaml logo %q is not a raster image: links to the site preview with no image (og:image wants a png, jpg, webp or gif)", cfg.Site.Logo))
		}
	}
	out = append(out, unresolvableRefs(cfg)...)
	out = append(out, missingAssets(cfg)...)
	out = append(out, iconSizeClaims(cfg)...)
	out = append(out, categoryConfusion(cfg)...)
	out = append(out, inertSettings(cfg)...)
	if slugs := config.CdnSlugs(cfg); len(slugs) > 0 {
		out = append(out, fmt.Sprintf("%d icons load from a CDN in visitors' browsers (%s); run cairn -emit-icons to self-host them", len(slugs), strings.Join(slugs, ", ")))
	}
	return out
}

// missingLocales lists the enabled locales a translated field says nothing
// for. Two shapes come back empty on purpose: a plain string, which covers
// every locale at once, and an unset field, which is absent rather than
// half-translated and has a documented fallback of its own.
func missingLocales(cfg *config.Config, l config.LString) []string {
	if len(l) == 0 || l[""] != "" {
		return nil
	}
	var missing []string
	for _, loc := range cfg.Site.Locales {
		if l[loc] == "" {
			missing = append(missing, loc)
		}
	}
	return missing
}

// missingTranslations lists every translatable field that covers some of
// the enabled locales but not all of them. A plain string covers them all.
func missingTranslations(cfg *config.Config) []string {
	var out []string
	warn := func(where string, l config.LString) {
		if missing := missingLocales(cfg, l); len(missing) > 0 {
			out = append(out, fmt.Sprintf("%s has no %s", where, strings.Join(missing, ", ")))
		}
	}
	// The title is the <title>, so a half-translated one puts the other
	// language in the browser tab, in the bookmark and in every og:title,
	// which are the three places nobody proofreads.
	warn("site title", cfg.Site.Title)
	warn("site tagline", cfg.Site.Tagline)
	warn("site about", cfg.Site.About)
	// Link labels are the operator's own words in the header and the footer,
	// on every page of the site rather than one. The url identifies the entry
	// because it is the only field guaranteed to be there and readable.
	for _, l := range cfg.Site.Links {
		warn(fmt.Sprintf("links entry %q label", l.URL), l.Label)
	}
	for _, l := range cfg.Site.Footer {
		warn(fmt.Sprintf("footer entry %q label", l.URL), l.Label)
	}
	for _, p := range cfg.Site.Pages {
		warn(fmt.Sprintf("page %q title", p.ID), p.Title)
		warn(fmt.Sprintf("page %q body", p.ID), p.Body)
		for i, s := range p.Sections {
			warn(fmt.Sprintf("page %q section %d title", p.ID, i+1), s.Title)
			warn(fmt.Sprintf("page %q section %d body", p.ID, i+1), s.Body)
		}
	}
	for _, c := range cfg.Categories {
		// A category with no name at all is not a missing translation: the id
		// becomes the heading in every locale, which is documented and often
		// what the operator wants. Only a name that answers for some locales
		// and not others is a gap, and missingLocales already stays quiet
		// about the unset field.
		warn(fmt.Sprintf("category %q name", c.ID), c.Name)
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

// partialStringOverrides finds a strings: entry that answers for some of the
// site's locales and not the rest.
//
// Str resolves the override per locale and falls through to the built-in
// table on a miss, so {nav.menu: {fr: Sommaire}} on a [fr, en] site gives the
// French pages the operator's word and the English pages cairn's. It is the
// one case where someone explicitly asked for different wording and got it on
// half their site, and nothing on either page shows which half.
func partialStringOverrides(cfg *config.Config) []string {
	known := map[string]bool{}
	for _, k := range config.StringKeys() {
		known[k] = true
	}
	keys := make([]string, 0, len(cfg.Site.Strings))
	for k := range cfg.Site.Strings {
		// A key cairn does not use is already reported by inertSettings as
		// doing nothing at all. Which locales it covers is beside the point
		// then, and two warnings over one line is how a check gets skimmed.
		if known[k] {
			keys = append(keys, k)
		}
	}
	// Map iteration is random, and output that reorders between runs cannot
	// be diffed by the pipeline an operator wires -check into.
	sort.Strings(keys)

	var out []string
	for _, k := range keys {
		if missing := missingLocales(cfg, cfg.Site.Strings[k]); len(missing) > 0 {
			out = append(out, fmt.Sprintf("strings key %q has no %s: those locales keep cairn's own wording while the rest use yours (name every locale, or write one plain string to cover them all)",
				k, strings.Join(missing, ", ")))
		}
	}
	return out
}

// unsupportedLocales names an enabled locale cairn has no interface for.
//
// Nothing refuses it and nothing reports it: the pages build, they carry the
// right lang attribute, and every word cairn contributes (the navigation, the
// buttons, the search messages) comes out English. The remedy is a strings:
// block, so the message says so rather than leaving the operator to guess
// that their only option is dropping the language.
func unsupportedLocales(cfg *config.Config) []string {
	builtin := map[string]bool{}
	for _, l := range config.BuiltinLocales() {
		builtin[l] = true
	}
	var out []string
	for _, loc := range cfg.Site.Locales {
		// Str looks the base language up after the full tag, so pt-BR finds
		// the pt table and is dressed in Portuguese. Testing the full tag
		// alone would warn about every regional variant of a language that
		// does ship, which is the opposite of helpful.
		base, _, _ := strings.Cut(loc, "-")
		if builtin[strings.ToLower(base)] {
			continue
		}
		out = append(out, fmt.Sprintf("locale %q has no built-in interface: its pages carry that lang and an English menu, buttons and search messages (cairn ships %s; a strings: block supplies the rest)",
			loc, strings.Join(config.BuiltinLocales(), ", ")))
	}
	return out
}

var mdImgRefRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

// mediaWarnings flags files nothing references, references naming no file,
// and files heavy enough to hurt the visitors of the pages that show them.
func mediaWarnings(cfg *config.Config, mediaDir string) []string {
	// Keyed by the file in media/, valued by where the reference was written,
	// so the missing-file warning below can name the page to go and fix.
	used := map[string]string{}
	// A file in media/ can be written two ways, and both are documented: the
	// bare name, and the /media/… path it is served at. Recording only the
	// first made -check announce a file as unreferenced while it sat on the
	// page, which is worse than saying nothing: it invites deleting it.
	markUsed := func(src, where string) {
		rel, ok := strings.CutPrefix(src, "/media/")
		if !ok {
			// A URL and a data: image are somebody else's to resolve, and
			// neither is a file this directory could be missing.
			if config.IsURLOrAbs(src) || uriRe.MatchString(src) {
				return
			}
			rel = src
		}
		if _, seen := used[rel]; !seen {
			used[rel] = where
		}
	}
	type text struct {
		where string
		body  config.LString
	}
	texts := []text{{"site about", cfg.Site.About}}
	for _, p := range cfg.Site.Pages {
		texts = append(texts, text{fmt.Sprintf("page %q body", p.ID), p.Body})
		for i, s := range p.Sections {
			texts = append(texts, text{fmt.Sprintf("page %q section %d", p.ID, i+1), s.Body})
		}
	}
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			texts = append(texts, text{fmt.Sprintf("service %q details", s.ID), s.Details})
			for _, img := range s.Images {
				markUsed(img.Src, fmt.Sprintf("service %q images", s.ID))
			}
		}
	}
	for _, t := range texts {
		for _, body := range t.body {
			for _, m := range mdImgRefRe.FindAllStringSubmatch(body, -1) {
				markUsed(m[1], t.where)
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
		if _, ok := used[rel]; !ok {
			out = append(out, fmt.Sprintf("media/%s is not referenced by any service or page", rel))
		}
		if info, ierr := d.Info(); ierr == nil && info.Size() > 1<<20 {
			out = append(out, fmt.Sprintf("media/%s weighs %.1f MB; visitors download it in full, consider a smaller export", rel, float64(info.Size())/(1<<20)))
		}
		return nil
	})
	out = append(out, danglingRefs(used, mediaDir)...)
	return out
}

// danglingRefs is the walk above run backwards: a reference that names no
// file, rather than a file nothing names.
//
// Load opens every images: entry and refuses to boot on a missing one, so an
// image written in markdown was the only way left to ship a 404. It reads
// exactly like the images: form in the documentation, it sits in about text,
// page bodies and service details, and nothing ever opened it.
func danglingRefs(used map[string]string, mediaDir string) []string {
	refs := make([]string, 0, len(used))
	for rel := range used {
		refs = append(refs, rel)
	}
	// Map iteration is random, and output that reorders between runs cannot be
	// diffed by the pipeline an operator wires -check into.
	sort.Strings(refs)

	var out []string
	for _, rel := range refs {
		// A browser normalises the .. away before it ever asks, so a stat of
		// what sits above media/ answers a question nobody posed.
		if rel == "" || strings.Contains(rel, "..") {
			continue
		}
		p := filepath.Join(mediaDir, filepath.FromSlash(rel))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			continue
		}
		out = append(out, fmt.Sprintf("%s shows %q, which is not there: the page renders a broken image (expected a file at %s)", used[rel], rel, p))
	}
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

// assetMount is the URL prefix the -assets directory is served at. A
// reference under it names a file the operator dropped there; a URL, a bare
// name and any other route are somebody else's to resolve.
const assetMount = "/assets/"

// assetsMounted reports whether the -assets directory is there to look in.
//
// -check on a laptop keeps the default /assets, which exists in the container
// and on nobody's machine, so every reference under it would come back
// missing at once. A page of warnings that are all wrong buries the one that
// is right and teaches the operator to stop reading the output, so with no
// directory to consult the checks below say nothing at all.
func assetsMounted() bool {
	st, err := os.Stat(config.AssetsPath)
	return err == nil && st.IsDir()
}

// assetPath maps an /assets/… reference to the file it names, and returns ""
// for anything that is not one. It refuses to climb out of the mount, the
// same rule the file server and the manifest measurement both apply: a stat
// of something outside the directory could only ever answer the wrong
// question, since a browser normalises the ".." away before it ever asks.
func assetPath(ref string) string {
	rel, ok := strings.CutPrefix(ref, assetMount)
	if !ok || rel == "" || strings.Contains(rel, "..") {
		return ""
	}
	return filepath.Join(config.AssetsPath, filepath.FromSlash(rel))
}

// missingAssets checks that every /assets/… reference reaches a file.
//
// Load already stats each media/ image, so a mistyped screenshot refuses to
// boot while a mistyped logo boots fine and paints a broken image on every
// page. These stay warnings for the same reason unresolvableRefs does: the
// damage is a missing picture, and refusing to boot over one would replace a
// working site with the getting-started page.
func missingAssets(cfg *config.Config) []string {
	if !assetsMounted() {
		return nil
	}
	// absent answers "this names a file in the mount and the file is not
	// there", and hands back the path so the message can name it: an operator
	// staring at a correct-looking /assets/logo.png needs to be told which
	// directory cairn looked in, which is the -assets flag they forgot.
	absent := func(ref string) (string, bool) {
		p := assetPath(ref)
		if p == "" {
			return "", false
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return "", false
		}
		return p, true
	}
	var out []string
	if p, miss := absent(cfg.Site.Logo); miss {
		out = append(out, fmt.Sprintf("site.yaml logo %q is not in the assets directory: the header paints a broken image and og:image sends every link preview to a 404 (expected a file at %s)", cfg.Site.Logo, p))
	}
	if p, miss := absent(cfg.Site.Favicon); miss {
		out = append(out, fmt.Sprintf("site.yaml favicon %q is not in the assets directory: the browser tab falls back to a blank page icon (expected a file at %s)", cfg.Site.Favicon, p))
	}
	for i, ic := range cfg.Site.Icons {
		if p, miss := absent(ic.Src); miss {
			out = append(out, fmt.Sprintf("site.yaml icons entry %d src %q is not in the assets directory: the manifest offers a home-screen icon that 404s, so the phone invents one (expected a file at %s)", i+1, ic.Src, p))
		}
	}
	for _, l := range cfg.Site.Links {
		if p, miss := absent(l.Icon); miss {
			out = append(out, fmt.Sprintf("site.yaml links entry %q icon %q is not in the assets directory: the link renders with a broken image beside its label (expected a file at %s)", l.URL, l.Icon, p))
		}
	}
	// footer icons are deliberately not checked here. inertSettings already
	// says the key is dropped and never rendered, so whether the file exists
	// changes nothing a visitor sees, and a second warning about one line is
	// exactly the noise that gets the first one ignored.
	return out
}

// measure reads a raster's real size. DecodeConfig only understands the
// formats something has registered, which is what the blank image/ imports at
// the top of this file are for: dropping one would not fail the build, it
// would quietly make every png unmeasurable and the check below silent.
//
// Anything it cannot decode comes back zero, which is the honest answer for
// all three ways that happens: an svg has no intrinsic size, a remote file
// would need an outbound request cairn does not make, and a file that is not
// there is unknown rather than wrong. Callers skip a zero instead of
// comparing against it.
func measure(path string) (int, int) {
	if path == "" {
		return 0, 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()
	c, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return c.Width, c.Height
}

// iconSizeClaims compares each icons entry with the file it points at.
//
// cairn cannot resize an image, so the operator states each size and the
// manifest publishes it as fact. A wrong number is invisible everywhere: the
// manifest validates, the file loads, and a phone simply picks the file for a
// slot it does not fill, which is how a home-screen icon comes out blurred.
// The measurement already exists for the favicon, and only this list went
// unchecked.
func iconSizeClaims(cfg *config.Config) []string {
	if !assetsMounted() {
		return nil
	}
	var out []string
	for i, ic := range cfg.Site.Icons {
		// "any" is a claim about a scalable file, not a measurement, so there
		// is nothing to disagree with. Everything else that cannot be
		// measured (an svg, a URL, a file that is not there) comes back zero
		// from measure and is skipped on the next line; the missing file has
		// its own warning already.
		if strings.Contains(ic.Sizes, "any") {
			continue
		}
		w, h := measure(assetPath(ic.Src))
		if w == 0 || h == 0 {
			continue
		}
		measured := fmt.Sprintf("%dx%d", w, h)
		// One file may legitimately declare several sizes: an ico holds a 16
		// and a 32, and DecodeConfig reports one of them. Any match is enough.
		matches := false
		for _, s := range strings.Fields(ic.Sizes) {
			if s == measured {
				matches = true
				break
			}
		}
		if matches {
			continue
		}
		out = append(out, fmt.Sprintf("site.yaml icons entry %d declares sizes %q but %s measures %s: the manifest states the declared size as fact, so a phone picks this file for a slot it does not fill (correct sizes, or supply a file that size)",
			i+1, ic.Sizes, ic.Src, measured))
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
