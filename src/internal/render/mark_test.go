package render

import (
	"regexp"
	"strings"
	"testing"
)

var shapeRE = regexp.MustCompile(`<(?:rect|circle|ellipse|polygon|path)\b[^>]*/?>`)

// cairn's mark is drawn twice: once in assets/favicon.svg, which becomes the
// tab icon and every generated png, and once inline in layout.tmpl, which is
// the icon a service falls back to when it declares none.
//
// They are two files, so they drift. They did: the mark was redrawn and the
// template kept the old five stones for a whole release cycle, which put the
// previous logo on every card without an icon while the tab showed the new
// one. Nothing failed, nothing warned. This is what warns.
func TestTheMarkIsDrawnTheSameInBothPlaces(t *testing.T) {
	favicon, err := Embedded.ReadFile("assets/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := Embedded.ReadFile("templates/layout.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	_, rest, ok := strings.Cut(string(layout), `{{define "glyph"}}`)
	if !ok {
		t.Fatal(`no {{define "glyph"}} in layout.tmpl`)
	}
	glyph, _, _ := strings.Cut(rest, "{{end}}")

	inFavicon := shapeRE.FindAllString(string(favicon), -1)
	inGlyph := shapeRE.FindAllString(glyph, -1)
	if len(inFavicon) == 0 {
		t.Fatal("no shapes found in favicon.svg")
	}
	if len(inFavicon) != len(inGlyph) {
		t.Fatalf("favicon.svg draws %d shapes, the glyph draws %d", len(inFavicon), len(inGlyph))
	}
	for i := range inFavicon {
		if inFavicon[i] != inGlyph[i] {
			t.Errorf("shape %d differs:\n  favicon.svg: %s\n  layout.tmpl: %s", i, inFavicon[i], inGlyph[i])
		}
	}
}
