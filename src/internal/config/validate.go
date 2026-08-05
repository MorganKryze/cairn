package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func validateSite(site *Site, serviceFiles map[string]string) error {
	if len(site.Locales) == 0 {
		site.Locales = []string{"en"}
	}
	for _, l := range site.Locales {
		if !localeRe.MatchString(l) {
			return fmt.Errorf("config: site.yaml: invalid locale %q (expected codes like fr, en, pt-BR)", l)
		}
	}
	if !accentRe.MatchString(site.Theme.Accent) {
		return fmt.Errorf("config: site.yaml: theme.accent %q is not a hex color (expected e.g. \"#247b7b\")", site.Theme.Accent)
	}
	if fam := site.Theme.Font.Family; fam != "" {
		if !fontFamilyRe.MatchString(fam) {
			return fmt.Errorf("config: site.yaml: theme.font.family %q contains characters a CSS font family cannot, and the value is inlined into the page's stylesheet: keep it to letters, digits, spaces, quotes, commas and hyphens (expected e.g. \"Inter, system-ui, sans-serif\")", fam)
		}
		if first := FirstFontFamily(fam); !isQuotedFontName(first) && strings.ContainsAny(first, " \t") {
			return fmt.Errorf("config: site.yaml: theme.font.family %q names its first family with a space unquoted, so the browser would read two families and never load the custom font (write it quoted, e.g. \"%s\")", fam, quotedFontName(first))
		}
	}
	if f := site.Theme.Font.File; f != "" {
		if site.Theme.Font.Family == "" {
			return fmt.Errorf("config: site.yaml: theme.font.file needs theme.font.family to name the font (the first family in the list is the one the file provides, which is how the page uses it)")
		}
		rel, ok := FontRef(f)
		if !ok || rel == "" {
			return fmt.Errorf("config: site.yaml: theme.font.file %q is not a file in the config directory's fonts/ folder (expected e.g. fonts/custom-font.woff2, served by cairn itself with no external request)", f)
		}
		if !fontFileRe.MatchString(rel) {
			return fmt.Errorf("config: site.yaml: theme.font.file %q carries characters a filename cannot, and the path is inlined into the page's stylesheet as the font's url(): keep it to letters, digits, dots, dashes and slashes (expected e.g. fonts/custom-font.woff2)", f)
		}
		if FontFormat(rel) == "" {
			return fmt.Errorf("config: site.yaml: theme.font.file %q does not end in a font extension cairn can serve (expected .woff2, .woff, .ttf or .otf)", f)
		}
	}
	if g := site.Status.Gatus; g != "" && !isHTTPURL(g) {
		return fmt.Errorf("config: site.yaml: status.gatus %q is not a URL (expected e.g. https://status.example.org)", g)
	}
	if err := validateStatusAddress(site); err != nil {
		return err
	}
	if p := site.Status.Page; p != "" && !isHTTPURL(p) {
		return fmt.Errorf("config: site.yaml: status.page %q is not a URL (expected e.g. https://status.example.org)", p)
	}
	if iv := site.Status.Interval; iv != "" {
		if d, err := time.ParseDuration(iv); err != nil || d < 5*time.Second {
			return fmt.Errorf("config: site.yaml: status.interval %q is not a duration of at least 5s (expected e.g. 60s)", iv)
		}
	}
	// status.ca is a trust anchor, so a value cairn cannot reach is refused
	// here rather than discovered on the first poll, where the whole failure
	// is grey pills and one log line. http stays allowed; the warning that
	// pays for it lives elsewhere.
	if ca := site.Status.CA; ca != "" {
		if err := checkLinkScheme("status.ca", ca, imageSchemes, "http://…, https://… or an /assets path"); err != nil {
			return err
		}
		if !isHTTPURL(ca) && AssetFile(ca) == "" {
			return fmt.Errorf("config: site.yaml: status.ca %q is neither a URL nor a file under the mounted assets dir (expected e.g. https://pki.example.org/ca.crt or /assets/ca.crt)", ca)
		}
	}
	// A hosting flag leads to a page cairn serves, to a path, or to a URL. A
	// page id is the one that follows the visitor's language and the one that
	// can be a typo, so a name matching no page stops the load and lists the
	// ones that do.
	for _, f := range []struct{ key, val string }{
		{"hosting_flag.self", site.HostingFlag.Self},
		{"hosting_flag.external", site.HostingFlag.External},
	} {
		if f.val == "" {
			continue
		}
		if err := checkLinkScheme(f.key, f.val, linkSchemes, "a page id, an absolute path or https://…"); err != nil {
			return err
		}
		if IsURLOrAbs(f.val) {
			continue
		}
		if !slices.ContainsFunc(site.Pages, func(p SitePage) bool { return p.ID == f.val }) {
			ids := make([]string, 0, len(site.Pages))
			for _, p := range site.Pages {
				ids = append(ids, p.ID)
			}
			if len(ids) == 0 {
				return fmt.Errorf("config: site.yaml: %s %q names a page, and this site has no pages (write one under pages:, or give a path or a URL instead)", f.key, f.val)
			}
			return fmt.Errorf("config: site.yaml: %s %q is not a page id (this site has %s), a path or a URL", f.key, f.val, strings.Join(ids, ", "))
		}
	}
	// logo and favicon are the two image fields nothing else validates: a value
	// that is not a URL only earns a -check warning, a missing file being a
	// plausible typo, while an executable scheme stops the load. The favicon
	// is the sharp case: it reaches the manifest as JSON, which no
	// html/template escaping covers, and comes back from /favicon.ico as a
	// Location header. Both halves of a themed pair reach both.
	for _, f := range append(site.Logo.Fields("logo"), site.Favicon.Fields("favicon")...) {
		if err := checkLinkScheme(f.Key, f.Val, imageSchemes, "https://… or an /assets path"); err != nil {
			return err
		}
	}
	if err := checkDarkImage("logo", site.Logo); err != nil {
		return err
	}
	if err := checkDarkImage("favicon", site.Favicon); err != nil {
		return err
	}
	// security.txt is a line-based format: a newline in any value would let a
	// typo forge a field.
	for _, f := range []struct{ key, val string }{
		{"security.contact", site.Security.Contact},
		{"security.policy", site.Security.Policy},
		{"security.encryption", site.Security.Encryption},
	} {
		if f.val == "" {
			continue
		}
		if strings.ContainsAny(f.val, "\r\n") {
			return fmt.Errorf("config: site.yaml: %s must be one line", f.key)
		}
		// uriRe takes any scheme, so it is the one URL check here an
		// executable scheme walks straight through. Catch it first. tel:
		// stays legal here and nowhere else: this file is plain text, so
		// nothing blanks it, and RFC 9116 names it.
		if err := checkContactScheme(f.key, f.val); err != nil {
			return err
		}
		if !uriRe.MatchString(f.val) {
			return fmt.Errorf("config: site.yaml: %s %q is not a URI (expected e.g. mailto:security@example.org or https://example.org/security)", f.key, f.val)
		}
	}
	if site.Security.Contact == "" && (site.Security.Policy != "" || site.Security.Encryption != "") {
		return fmt.Errorf("config: site.yaml: security needs a contact (expected: security: {contact: mailto:security@example.org})")
	}
	site.URL = strings.TrimSuffix(site.URL, "/")
	if u := site.URL; u != "" && !isHTTPURL(u) {
		return fmt.Errorf("config: site.yaml: url %q is not a URL (expected e.g. https://tools.example.org)", u)
	}
	// url is the domain, never the address of the page. cairn appends
	// -base-path itself, so a path written here is carried twice: an operator
	// serving example.org/cairn who writes the full public URL ends up with
	// canonical links, hreflang alternates and a whole sitemap pointing at
	// example.org/cairn/cairn/. Search engines act on it, so this is an error
	// rather than a warning: a fresh start then serves the getting-started
	// page with the reason logged, and a reload keeps the previous pages.
	if u, err := url.Parse(site.URL); site.URL != "" && err == nil && strings.Trim(u.Path, "/") != "" {
		return fmt.Errorf("config: site.yaml: url %q must be the domain alone, with no path (serving under example.org/cairn is what -base-path is for, and cairn adds that prefix itself)", site.URL)
	}
	for _, l := range site.Links {
		if len(l.Label) == 0 || l.URL == "" {
			return fmt.Errorf("config: site.yaml: every links entry needs label and url (expected: - {label: Wiki, url: https://…})")
		}
		// mailto: is a legitimate target here, so this field gets the link
		// list rather than the image one.
		if err := checkLinkScheme("links url", l.URL, linkSchemes, "https://…, mailto:… or an absolute path"); err != nil {
			return err
		}
		if ic := l.Icon; ic != "" && !IsURLOrAbs(ic) {
			// Any scheme at all lands here, being neither a URL nor an /assets
			// path, so this refusal already covers the schemes checked above.
			if _, ok := Glyphs[ic]; !ok {
				return fmt.Errorf("config: site.yaml: links icon %q is not a built-in glyph (%s), a URL or an /assets path", ic, strings.Join(GlyphNames(), ", "))
			}
		}
	}
	// A footer entry needs what a links entry needs: no url renders an empty
	// href, a link that looks live and reloads the page, and no label renders
	// nothing visible at all.
	for _, l := range site.Footer {
		if len(l.Label) == 0 || l.URL == "" {
			return fmt.Errorf("config: site.yaml: every footer entry needs label and url (expected: - {label: Legal, url: /legal})")
		}
		// icon is a header-link key. A footer entry renders none, and
		// schema/site.json does not list it, so accepting one had the editor
		// and the loader saying different things. The refusal names the list
		// where an icon does work.
		if l.Icon != "" {
			return fmt.Errorf("config: site.yaml: footer entry %q has an icon: only header links render one (move the entry to links, or drop the icon)", l.URL)
		}
		if err := checkLinkScheme("footer url", l.URL, linkSchemes, "https://…, mailto:… or an absolute path"); err != nil {
			return err
		}
	}
	for i, ic := range site.Icons {
		where := fmt.Sprintf("icons entry %d", i+1)
		if ic.Src == "" || ic.Sizes == "" {
			return fmt.Errorf("config: site.yaml: %s needs src and sizes "+
				"(expected: - {src: /assets/icon-512.png, sizes: 512x512})", where)
		}
		if !IsURLOrAbs(ic.Src) {
			return fmt.Errorf("config: site.yaml: %s src %q is not a URL or an /assets path", where, ic.Src)
		}
		if !iconSizesRe.MatchString(ic.Sizes) {
			return fmt.Errorf("config: site.yaml: %s sizes %q is not a size "+
				"(expected e.g. 512x512, or \"any\" for an svg)", where, ic.Sizes)
		}
		for _, p := range strings.Fields(ic.Purpose) {
			if p != "any" && p != "maskable" && p != "monochrome" {
				return fmt.Errorf("config: site.yaml: %s purpose %q is not one of "+
					"any, maskable, monochrome (they may be combined: \"any maskable\")", where, p)
			}
		}
	}
	pageIDs := map[string]bool{}
	for _, p := range site.Pages {
		switch {
		case !idRe.MatchString(p.ID):
			return fmt.Errorf("config: site.yaml: invalid page id %q, ids become URLs (expected lowercase letters, digits and dashes, e.g. legal)", p.ID)
		case len(p.Title) == 0 || (len(p.Body) == 0 && len(p.Sections) == 0):
			return fmt.Errorf("config: site.yaml: page %q needs a title and a body or sections", p.ID)
		case pageIDs[p.ID]:
			return fmt.Errorf("config: site.yaml: duplicate page id %q", p.ID)
		case serviceFiles[p.ID] != "":
			return fmt.Errorf("config: site.yaml: page id %q collides with the service id defined in %s", p.ID, serviceFiles[p.ID])
		}
		pageIDs[p.ID] = true
		for _, s := range p.Sections {
			if len(s.Title) == 0 || len(s.Body) == 0 {
				return fmt.Errorf("config: site.yaml: page %q: every section needs title and body", p.ID)
			}
		}
	}
	return nil
}

// validateStatusAddress settles which monitor cairn polls and where.
//
// Two keys can name it and they must not both be taken: status.gatus, which
// every site running today uses and which implies its own provider, and
// status.url with status.provider for a monitor that is not Gatus. Each
// ambiguity below is refused with the key to fix rather than resolved by
// guessing, since a wrong guess draws pills from an address the operator did
// not mean.
func validateStatusAddress(site *Site) error {
	st := &site.Status
	if st.Provider != "" && !slices.Contains(StatusProviders(), st.Provider) {
		return fmt.Errorf("config: site.yaml: status.provider %q is not a monitor cairn reads (expected one of %s)", st.Provider, strings.Join(StatusProviders(), ", "))
	}
	if st.Gatus != "" && st.URL != "" {
		return fmt.Errorf("config: site.yaml: set one of status.gatus and status.url, not both (they are two ways of naming the one monitor cairn polls; status.gatus is the Gatus spelling and needs no status.provider)")
	}
	if st.Gatus != "" && st.Provider != "" && st.Provider != "gatus" {
		return fmt.Errorf("config: site.yaml: status.gatus names a Gatus instance, so status.provider: %s contradicts it (use status.url with status.provider: %s)", st.Provider, st.Provider)
	}
	if st.URL != "" {
		if st.Provider == "" {
			return fmt.Errorf("config: site.yaml: status.url needs status.provider to say what answers there (expected one of %s)", strings.Join(StatusProviders(), ", "))
		}
		if !isHTTPURL(st.URL) {
			return fmt.Errorf("config: site.yaml: status.url %q is not a URL (expected e.g. https://status.example.org)", st.URL)
		}
	}
	// Kuma reads by status page and has no endpoint listing every monitor, so
	// the slug is half the address rather than an extra. Anywhere else it is a
	// key that would silently do nothing.
	switch p := site.StatusProvider(); {
	case p == "kuma":
		if st.Slug == "" && st.URL != "" {
			return fmt.Errorf("config: site.yaml: status.slug is needed with status.provider: kuma (kuma serves statuses per published status page; the slug is the last part of its URL, as in https://kuma.example.org/status/tools)")
		}
	case st.Slug != "" && p != "":
		return fmt.Errorf("config: site.yaml: status.slug %q means nothing to status.provider: %s (only kuma reads by status page)", st.Slug, p)
	}
	// A mapping is the whole configuration of the json provider and means
	// nothing to any other. Both halves are refused here rather than left to
	// -check: a mapping that silently does nothing is what this file exists to
	// catch.
	noMapping := st.Map.List == "" && st.Map.Key == "" && st.Map.State == "" &&
		len(st.Map.Up) == 0 && len(st.Map.Degraded) == 0 && len(st.Map.Maintenance) == 0 &&
		len(st.Map.Unknown) == 0
	switch p := site.StatusProvider(); {
	case p == "json":
		if noMapping && st.URL != "" {
			return fmt.Errorf("config: site.yaml: status.map is needed with status.provider: json (it says how to read the document: list, the path to the array; key and state, the fields of each element; then up, degraded and maintenance, the values that mean each)")
		}
		for _, f := range []struct{ key, val string }{{"key", st.Map.Key}, {"state", st.Map.State}} {
			if !noMapping && f.val == "" {
				return fmt.Errorf("config: site.yaml: status.map.%s is needed (it names the field of each element holding the service %s; status.map.list is the only optional path, and empty means the document is the array)", f.key, f.key)
			}
		}
	case !noMapping && p != "":
		return fmt.Errorf("config: site.yaml: status.map means nothing to status.provider: %s (only json reads a document through a mapping)", p)
	}
	// The assets directory is served to every visitor: GET /assets/token.txt
	// on a running cairn returns the file. status.ca under /assets stays legal,
	// a CA certificate being the public half of an authority where a token is
	// the public half of nothing.
	if tf := st.TokenFile; tf != "" {
		if AssetFile(tf) != "" || strings.HasPrefix(tf, "/assets/") {
			return fmt.Errorf("config: site.yaml: status.token_file %q is inside the assets directory, which cairn serves to every visitor: the token would be published rather than stored (mount it somewhere else, as /run/secrets/status-token)", tf)
		}
		if !filepath.IsAbs(tf) {
			return fmt.Errorf("config: site.yaml: status.token_file %q is not an absolute path (cairn resolves it against nothing: name the file where the platform mounts it, as /run/secrets/status-token)", tf)
		}
	}
	return nil
}

// The schemes cairn will actually emit. html/template puts http, https,
// mailto and paths in an href or a src, and replaces every other scheme with
// #ZgotmplZ, so a value carrying one renders as a link that goes nowhere and
// says nothing. tel: fails exactly the way javascript: does, and as quietly.
//
// An allow-list rather than a list of the dangerous ones: a deny-list is only
// as current as the last time somebody thought about it, and the question here
// is which schemes cairn can emit, not which ones are hostile.
var (
	linkSchemes = []string{"http", "https", "mailto"}
	// An image is a file, so mailto: has no business naming one.
	imageSchemes = []string{"http", "https"}
	// security.txt is the exception, and the reason the allow-list above is
	// scoped to links rather than applied to the whole file: that file is
	// plain text, never markup, so nothing blanks anything on the way out and
	// RFC 9116 names tel: itself. Only the schemes a browser can be talked
	// into executing are refused there, data: among them, since data:text/html
	// carries a whole document, script and all.
	unsafeSchemes = []string{"javascript", "vbscript", "data"}
)

// urlScheme returns the scheme a browser would read from s, lowercased, and ""
// when there is none, which is every relative and root-absolute path.
//
// A browser removes every tab, newline and carriage return from a URL and
// strips leading control characters and spaces before it looks at the scheme,
// so "java\tscript:alert(1)", " javascript:…" and "JavaScript:…" are all the
// same URL as the plain one. Matching the literal text would refuse the
// obvious spelling and pass the three that matter.
func urlScheme(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimLeftFunc(s, func(r rune) bool { return r <= ' ' })
	i := strings.IndexByte(s, ':')
	// A colon past a slash, a question mark or a hash belongs to the path or
	// the query: "/assets/a:b.png" is a path and always was.
	if i < 0 || strings.ContainsAny(s[:i], "/?#") {
		return ""
	}
	return strings.ToLower(s[:i])
}

// checkLinkScheme refuses a value cairn cannot put in an href or a src. An
// error rather than a warning: the alternative is a page that renders, a link
// that does nothing when clicked, and no log line mentioning it.
func checkLinkScheme(field, val string, allowed []string, accepted string) error {
	sc := urlScheme(val)
	if sc == "" || slices.Contains(allowed, sc) {
		return nil
	}
	return fmt.Errorf("config: site.yaml: %s %q uses the %s: scheme, which cairn does not emit: the link would render dead and nothing would say why (expected %s)", field, val, sc, accepted)
}

// checkContactScheme is the security.txt half: any URI but an executable one.
func checkContactScheme(field, val string) error {
	if sc := urlScheme(val); slices.Contains(unsafeSchemes, sc) {
		return fmt.Errorf("config: site.yaml: %s %q uses the %s: scheme, which no researcher can act on (expected e.g. mailto:security@example.org, https://example.org/security or tel:+33…)", field, val, sc)
	}
	return nil
}

// isQuotedFontName reports whether a CSS family name carries its own quotes,
// which a name with spaces needs before it can stand alone in an @font-face.
func isQuotedFontName(name string) bool {
	return len(name) >= 2 &&
		(name[0] == '"' && name[len(name)-1] == '"' ||
			name[0] == '\'' && name[len(name)-1] == '\'')
}

func quotedFontName(name string) string {
	if isQuotedFontName(name) {
		return name
	}
	return `"` + name + `"`
}

// checkDarkImage guards the half of a themed image that leaves the markup
// behind. The light one goes into an src attribute, where html/template
// escapes it; the dark one is written into the page's inline stylesheet as the
// url() of a content rule, which nothing escapes.
//
// Two characters are all it takes: a path that is not a climb, names no other
// origin and ends in .svg can still carry "</style>", and the browser ends the
// style element there and reads the rest as markup. Only the dark half is held
// to this, so nothing an existing site serves changes meaning.
func checkDarkImage(key string, ref ThemedRef) error {
	if !ref.Themed() {
		return nil
	}
	if strings.ContainsAny(ref.Dark, stylesheetBreakers) {
		return fmt.Errorf("config: site.yaml: %s.dark %q carries %q or %q, and the dark image is written into the page's stylesheet where nothing escapes it: an image URL has no use for either (expected e.g. /assets/logo-white.svg)", key, ref.Dark, "<", ">")
	}
	return nil
}
