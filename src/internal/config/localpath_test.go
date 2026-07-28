package config

import "testing"

// A leading slash is not enough to call something local. "//cdn.example.org/x"
// is a protocol-relative URL: a browser resolves it against another origin,
// and browsers normalise the backslash form to the same thing.
//
// Getting this wrong was not theoretical. Every caller that prefixes a base
// path or a site URL tested for a bare leading slash, so a logo written the
// protocol-relative way came out as "/cairn//cdn.example.org/logo.png", a path
// that resolves nowhere. CodeQL flags the same predicate as a bad redirect
// check; nothing reachable from a visitor's request ever reached it, but the
// distinction it asks for is the one that fixes the real defect.
func TestIsLocalPathSeparatesTheTwoKindsOfSlash(t *testing.T) {
	for _, c := range []struct {
		in    string
		local bool
		pass  bool // accepted by IsURLOrAbs, i.e. a link rather than a slug
	}{
		{"/assets/logo.png", true, true},
		{"/", true, true},
		{"//cdn.example.org/logo.png", false, true}, // another origin
		{`/\cdn.example.org/logo.png`, false, true}, // the form browsers normalise
		{"///cdn.example.org", false, true},         //
		{"https://cdn.example.org/x", false, true},  //
		{"http://cdn.example.org/x", false, true},   //
		{"jellyfin", false, false},                  // a slug: neither
		{"", false, false},                          //
		{"assets/logo.png", false, false},           // relative, not root-absolute
		{"HTTPS://cdn.example.org/x", false, false}, // scheme match is exact, as it always was
	} {
		if got := IsLocalPath(c.in); got != c.local {
			t.Errorf("IsLocalPath(%q) = %v, want %v", c.in, got, c.local)
		}
		if got := IsURLOrAbs(c.in); got != c.pass {
			t.Errorf("IsURLOrAbs(%q) = %v, want %v", c.in, got, c.pass)
		}
	}
}

// The slug path is what decides between the operator's own file, the CDN, and
// passing a link straight through. A protocol-relative icon is a link.
func TestIconURLPassesLinksThrough(t *testing.T) {
	cfg := &Config{LocalIcons: map[string]string{"jellyfin": "/assets/icons/jellyfin.svg"}}
	for _, c := range []struct{ in, want string }{
		{"", ""},
		{"jellyfin", "/assets/icons/jellyfin.svg"},                         // the operator's own
		{"grafana", iconCDN + "grafana.svg"},                               // the CDN
		{"/assets/mine.svg", "/assets/mine.svg"},                           // a local file
		{"https://cdn.example.org/i.svg", "https://cdn.example.org/i.svg"}, // a URL
		{"//cdn.example.org/i.svg", "//cdn.example.org/i.svg"},             // still a URL
	} {
		if got := IconURL(cfg, c.in); got != c.want {
			t.Errorf("IconURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
