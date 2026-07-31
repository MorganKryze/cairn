package render

// cairn renders the long text fields (about, pages, details) with goldmark:
// CommonMark plus the GFM extensions (tables, strikethrough, autolinks, task
// lists). Three things are narrowed from goldmark's defaults on purpose, and
// each has its own test because losing one silently is the failure mode here:
//
//   - Raw HTML is not a syntax. The HTML block parser and the inline raw-HTML
//     parser are taken out of the parser, so "<b>x</b>" written in a config
//     field is ordinary text and comes out escaped, rather than as markup
//     (goldmark with WithUnsafe) or as goldmark's "<!-- raw HTML omitted -->"
//     comment, which would swallow what the operator wrote. A config file can
//     never break the page or the CSP.
//   - Link schemes are an allow-list, not goldmark's deny-list. goldmark
//     blanks the href of javascript:, vbscript:, file: and non-image data:
//     URLs and lets everything else through, so ftp:, tel:, chrome: or any
//     invented scheme would become live links. cairn keeps what a directory
//     page has a reason to link: http, https, mailto and anything scheme-less
//     (paths, anchors, relative). Anything else renders as the words the
//     writer wrote, with no anchor around them.
//   - An image alone in a block is a <figure class="shot"> carrying the width
//     and height the media resolver knows, so the page does not jump while the
//     image loads. goldmark would render a bare inline <img> inside a <p>.
//
// Everything else is goldmark's own output, which is the point of the change:
// headings, ordered and nested lists, blockquotes, code blocks, rules and
// tables used to render as literal text.

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// mdCtx carries the per-render options: the css class of plain paragraphs
// and the image resolver (src -> url, width, height; 0x0 when unknown).
type mdCtx struct {
	pClass string
	media  func(string) (string, int, int)
}

// mdCtxKey hands one mdCtx to the AST transformer. The goldmark instance is
// built once and shared, so the per-field options travel with the parse rather
// than with the parser, and the transformer resolves media sizes and the
// paragraph class into the tree while it still has them.
var mdCtxKey = parser.NewContextKey()

// md is the shared converter. The custom renderer sits at priority 100 against
// goldmark's own at 1000: the lowest number registers last and wins.
var md = newMarkdown()

// mdParserOptions is the syntax cairn accepts, with the two raw-HTML parsers
// removed so HTML is text rather than markup. Shared, so the parser that finds
// image references cannot drift from the one that renders them.
func mdParserOptions() []parser.Option {
	return []parser.Option{
		parser.WithBlockParsers(mdWithout(parser.DefaultBlockParsers(), parser.NewHTMLBlockParser())...),
		parser.WithInlineParsers(mdWithout(parser.DefaultInlineParsers(), parser.NewRawHTMLParser())...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	}
}

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParser(parser.NewParser(mdParserOptions()...)),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(mdTransformer{}, 100))),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(mdRenderer{}, 100)),
			// A table column with an alignment renders style="text-align:…" by
			// default, and cairn's CSP is style-src 'self' with one hash for
			// the accent block: the browser would drop that attribute and log
			// a violation on every page. The align attribute says the same
			// thing, is not a style, and is what CSP has no opinion about.
			extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute),
		),
		goldmark.WithExtensions(extension.GFM),
	)
}

// mdWithout drops one parser from a default list. goldmark hands out the same
// pointer every time for these, so identity is enough and the rest of the list
// stays whatever the version we build against says it is: a parser added
// upstream keeps working, and only the one named here disappears.
func mdWithout(vs []util.PrioritizedValue, drop any) []util.PrioritizedValue {
	out := make([]util.PrioritizedValue, 0, len(vs))
	for _, v := range vs {
		if v.Value != drop {
			out = append(out, v)
		}
	}
	return out
}

// mdBlocks renders a field to one HTML string per top-level block. The
// templates iterate the slice, so a block is still the unit: rendering each
// child of the document on its own keeps that shape without a second parse.
func mdBlocks(src string, ctx mdCtx) []template.HTML {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	source := []byte(src)
	pc := parser.NewContext()
	pc.Set(mdCtxKey, ctx)
	doc := md.Parser().Parse(text.NewReader(source), parser.WithContext(pc))

	var out []template.HTML
	var buf bytes.Buffer
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		buf.Reset()
		if err := md.Renderer().Render(&buf, source, n); err != nil {
			continue
		}
		// goldmark ends a block with a newline; the prose template adds its own.
		if h := strings.TrimRight(buf.String(), "\n"); h != "" {
			out = append(out, template.HTML(h))
		}
	}
	return out
}

// mdSafeURL allows the schemes a directory page has a reason to link:
// anything scheme-less (paths, anchors, relative), http(s) and mailto.
//
// This is the whole link policy. goldmark's own IsDangerousURL is a deny-list
// and would pass ftp:, tel:, chrome: or jar: straight through, so cairn checks
// first and goldmark's check never gets to matter.
func mdSafeURL(u string) bool {
	l := strings.ToLower(u)
	for _, p := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return !strings.Contains(u, ":")
}

// mdFigure is what an image alone in a block becomes. The url and the size are
// resolved while the transformer still holds the mdCtx; the node's children
// are the caption, so it keeps whatever inline markup the caption was written
// with.
type mdFigure struct {
	ast.BaseBlock
	URL  string
	W, H int
}

var mdKindFigure = ast.NewNodeKind("CairnFigure")

func (*mdFigure) Kind() ast.NodeKind { return mdKindFigure }

func (n *mdFigure) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type mdTransformer struct{}

func (mdTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	ctx, _ := pc.Get(mdCtxKey).(mdCtx)

	// Whole-block images become figures, and every other top-level paragraph
	// takes the field's paragraph class. Both are top-level only: a paragraph
	// inside a list item is not the page intro.
	for n := doc.FirstChild(); n != nil; {
		next := n.NextSibling()
		if p, ok := n.(*ast.Paragraph); ok {
			if fig := mdFigureOf(p, ctx); fig != nil {
				doc.ReplaceChild(doc, p, fig)
			} else if ctx.pClass != "" {
				p.SetAttributeString("class", []byte(ctx.pClass))
			}
		}
		n = next
	}

	// A level-one heading becomes a level two.
	//
	// The page already has its <h1>: the site title on the home page, the page
	// title on a hosted one, the service name on a detail page. A second one
	// breaks the document outline, and a screen reader user navigating by
	// heading level lands on two competing tops with no way to tell which is
	// the page.
	//
	// This is not cairn overruling the operator. Someone writing "# Notes" in a
	// page body wants a heading for their text, not a second title for the
	// document, and the title they did ask for is the one in the config. Only
	// the top level moves, so "##" still renders exactly what it always did and
	// no existing page changes shape. -check says so, because two levels
	// rendering identically is otherwise a puzzle rather than a rule.
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && h.Level == 1 {
			h.Level = 2
		}
		return ast.WalkContinue, nil
	})

	// Images still inline after that pass resolve the same way, so an image in
	// the middle of a sentence points at the same file as one on its own line
	// and carries the same anti-jump attributes. The walk only collects: the
	// refused ones are edited out of the tree afterwards, not underneath it.
	var imgs []*ast.Image
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering {
			imgs = append(imgs, img)
		}
		return ast.WalkContinue, nil
	})
	for _, img := range imgs {
		url, w, h := mdMedia(string(img.Destination), ctx)
		if url == "" {
			mdUnwrap(img)
			continue
		}
		img.Destination = []byte(url)
		img.SetAttributeString("loading", []byte("lazy"))
		if w > 0 {
			img.SetAttributeString("width", []byte(strconv.Itoa(w)))
			img.SetAttributeString("height", []byte(strconv.Itoa(h)))
		}
	}
}

// mdFigureOf turns a paragraph holding one image and nothing else into a
// figure, and answers nil for anything else, including an image the resolver
// refused: that one falls through to mdUnwrap and keeps its caption as text,
// the same bargain a refused link gets.
func mdFigureOf(p *ast.Paragraph, ctx mdCtx) *mdFigure {
	if p.ChildCount() != 1 {
		return nil
	}
	img, ok := p.FirstChild().(*ast.Image)
	if !ok {
		return nil
	}
	fig := &mdFigure{}
	fig.URL, fig.W, fig.H = mdMedia(string(img.Destination), ctx)
	if fig.URL == "" {
		return nil
	}
	for c := img.FirstChild(); c != nil; {
		next := c.NextSibling()
		img.RemoveChild(img, c)
		fig.AppendChild(fig, c)
		c = next
	}
	return fig
}

// mdUnwrap puts a node's children where the node was, then drops the node.
func mdUnwrap(n ast.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	for c := n.LastChild(); c != nil; c = n.LastChild() {
		n.RemoveChild(n, c)
		parent.InsertAfter(parent, n, c)
	}
	parent.RemoveChild(parent, n)
}

// mdMedia resolves an image source. The resolver answers with a local /media/
// path and is trusted; without one (mdText, and tests) the source is still an
// operator string and goes through the same scheme allow-list as a link.
func mdMedia(src string, ctx mdCtx) (string, int, int) {
	if ctx.media != nil {
		return ctx.media(src)
	}
	if !mdSafeURL(src) {
		return "", 0, 0
	}
	return src, 0, 0
}

type mdRenderer struct{}

func (r mdRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
	reg.Register(mdKindFigure, r.renderFigure)
	reg.Register(east.KindTable, r.renderTable)
}

// renderTable wraps the table in the element that scrolls, instead of making
// the table itself scroll.
//
// The one-line way to stop a wide table pushing the page sideways is
// display:block on the table, and it costs more than it looks: Chrome and
// Safari drop the table role from the accessibility tree with it, so the rows
// and cells stop being announced as a table at all and a screen reader reads
// the cells as loose text. A wrapper keeps display:table and its semantics.
//
// tabindex is not decoration either. A region that scrolls has to be
// scrollable by keyboard, and a div with overflow is not focusable on its own,
// so someone who cannot use a pointer would never reach the right-hand columns.
func (mdRenderer) renderTable(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="table-wrap" tabindex="0"><table>`)
	} else {
		_, _ = w.WriteString("</table></div>")
	}
	return ast.WalkContinue, nil
}

func (mdRenderer) renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	// The children still render, so a refused link keeps its words and loses
	// only the anchor. Destination is the decoded target: goldmark resolves
	// "&#106;avascript:" before it reaches here, so the allow-list sees the
	// scheme the browser would see.
	if !mdSafeURL(string(n.Destination)) {
		return ast.WalkContinue, nil
	}
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		_, _ = w.Write(util.EscapeHTML(n.Title))
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	return ast.WalkContinue, nil
}

func (mdRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	if !entering {
		return ast.WalkContinue, nil
	}
	label := util.EscapeHTML(n.Label(source))
	url := n.URL(source)
	if n.AutoLinkType == ast.AutoLinkEmail && !bytes.HasPrefix(bytes.ToLower(url), []byte("mailto:")) {
		url = append([]byte("mailto:"), url...)
	}
	// "<javascript:alert(1)>" is a valid CommonMark autolink, so the same
	// allow-list runs here and not only on [text](url).
	if !mdSafeURL(string(url)) {
		_, _ = w.Write(label)
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape(url, false)))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(label)
	_, _ = w.WriteString("</a>")
	return ast.WalkContinue, nil
}

func (mdRenderer) renderFigure(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*mdFigure)
	if !entering {
		if n.HasChildren() {
			_, _ = w.WriteString("</figcaption>")
		}
		_, _ = w.WriteString("</figure>")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<figure class="shot"><img src="` + html.EscapeString(n.URL) + `" alt="" loading="lazy"`)
	// Width and height are what stop the page shifting as images arrive; they
	// are left out rather than guessed when the resolver does not know them.
	if n.W > 0 {
		_, _ = fmt.Fprintf(w, ` width="%d" height="%d"`, n.W, n.H)
	}
	_, _ = w.WriteString(">")
	if n.HasChildren() {
		_, _ = w.WriteString("<figcaption>")
	}
	return ast.WalkContinue, nil
}

var mdTagRe = regexp.MustCompile(`<[^>]*>`)

// mdText renders a block to plain text, for meta descriptions. goldmark puts
// newlines between blocks and between list items, so dropping the tags and
// collapsing what is left keeps words apart without inventing a space in the
// middle of a sentence.
func mdText(s string) string {
	var b strings.Builder
	for i, blk := range mdBlocks(s, mdCtx{}) {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(string(blk))
	}
	return strings.Join(strings.Fields(html.UnescapeString(mdTagRe.ReplaceAllString(b.String(), ""))), " ")
}

// mdRefParser has the syntax and none of the transformations. ImageRefs wants
// the images the operator wrote, before the resolver has had a chance to
// refuse one, and running the render parser here finds nothing at all: with no
// media resolver every image is unwrapped out of the tree before a walk can
// see it. Measured, not assumed.
var mdRefParser = parser.NewParser(mdParserOptions()...)

// HasTopHeading reports whether a field was written with a level-one heading,
// either "# Title" or the setext underline. It reads the untransformed tree, so
// it sees what the operator wrote rather than the h2 the renderer emits.
func HasTopHeading(src string) bool {
	if src == "" {
		return false
	}
	doc := mdRefParser.Parse(text.NewReader([]byte(src)), parser.WithContext(parser.NewContext()))
	found := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && h.Level == 1 {
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return found
}

// ImageRefs lists every image destination a markdown field references, in the
// order it references them.
//
// It exists so -check and the renderer cannot disagree about what an image is.
// The check used to run its own regexp, `!\[…\]\(…\)`, which knows only the
// parenthesised spelling: `![cap][ref]` with `[ref]: gone.png` underneath
// rendered two broken images and -check still printed ok. Asking the parser
// that actually renders the page is the only way that stays true as the
// markdown grows, and it costs nothing, since the parse is the same one.
//
// Destinations are returned raw, exactly as written. Resolving one against
// media/ or the assets mount is the caller's business, and cairn resolves them
// differently in different places.
func ImageRefs(src string) []string {
	if src == "" {
		return nil
	}
	doc := mdRefParser.Parse(text.NewReader([]byte(src)), parser.WithContext(parser.NewContext()))
	var out []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if img, ok := n.(*ast.Image); ok {
			out = append(out, string(img.Destination))
		}
		return ast.WalkContinue, nil
	})
	return out
}
