package render

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// testMedia is the resolver the render pipeline hands to mdBlocks: one image
// whose size is known, everything else resolved but unmeasured.
func testMedia(src string) (string, int, int) {
	if src == "plan.png" {
		return "/media/plan.png", 800, 600
	}
	return mediaURL(src), 0, 0
}

// mdOne renders src and returns its single block, so a test can say what it
// means about one piece of markdown without indexing a slice.
func mdOne(t *testing.T, src string, ctx mdCtx) string {
	t.Helper()
	blocks := mdBlocks(src, ctx)
	if len(blocks) != 1 {
		t.Fatalf("mdBlocks(%q) = %d blocks, want 1: %q", src, len(blocks), blocks)
	}
	return string(blocks[0])
}

// mdPara renders src and returns what is inside its <p>, which is where the
// inline rules show.
func mdPara(t *testing.T, src string) string {
	t.Helper()
	got := mdOne(t, src, mdCtx{media: testMedia})
	if !strings.HasPrefix(got, "<p>") || !strings.HasSuffix(got, "</p>") {
		t.Fatalf("mdBlocks(%q) = %q, want a paragraph", src, got)
	}
	return strings.TrimSuffix(strings.TrimPrefix(got, "<p>"), "</p>")
}

func TestMarkdownInline(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"plain text", "plain text"},
		{"**bold** and *italic*", "<strong>bold</strong> and <em>italic</em>"},
		{"a `code` span", "a <code>code</code> span"},
		{"[write me](mailto:a@b.org)", `<a href="mailto:a@b.org">write me</a>`},
		{"[docs](https://example.org/d)", `<a href="https://example.org/d">docs</a>`},
		{"[legal](/fr/legal/)", `<a href="/fr/legal/">legal</a>`},
		{"[bad](javascript:alert(1))", "bad"},
		{"**[bold link](https://x.org)**", `<strong><a href="https://x.org">bold link</a></strong>`},
		{"5 < 6 & 7 > 2", "5 &lt; 6 &amp; 7 &gt; 2"},
		{"`<script>`", "<code>&lt;script&gt;</code>"},
		{"a lone * star", "a lone * star"},
		{"unclosed **bold", "unclosed **bold"},
		{"unclosed [link(https://x", "unclosed [link(https://x"},
		// A link title used to break the href it was written next to.
		{`[x](https://x.org "the docs")`, `<a href="https://x.org" title="the docs">x</a>`},
	} {
		if got := mdPara(t, tc.in); got != tc.want {
			t.Errorf("inline %q = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMarkdownBlocks(t *testing.T) {
	src := "## Contact\n\nWrite to [me](mailto:a@b.org).\n\n- one\n- two\n\n![The plan](plan.png)"
	blocks := mdBlocks(src, mdCtx{media: testMedia})
	if len(blocks) != 4 {
		t.Fatalf("blocks = %d, want 4", len(blocks))
	}
	for i, want := range []string{
		"<h2>Contact</h2>",
		`<p>Write to <a href="mailto:a@b.org">me</a>.</p>`,
		"<ul>\n<li>one</li>\n<li>two</li>\n</ul>",
		`<figure class="shot"><img src="/media/plan.png" alt="" loading="lazy" width="800" height="600"><figcaption>The plan</figcaption></figure>`,
	} {
		if string(blocks[i]) != want {
			t.Errorf("block %d = %q, want %q", i, blocks[i], want)
		}
	}
	// A field with nothing in it renders no blocks at all, which is what the
	// templates test to decide whether the section exists.
	if got := mdBlocks("  \n\n ", mdCtx{}); got != nil {
		t.Errorf("mdBlocks(blank) = %q, want nil", got)
	}
}

// pClass is what puts .page-intro on a static page's opening text. It belongs
// to plain paragraphs and to nothing else: a heading in the same field is
// still a heading, and a paragraph nested in a list item is not the intro.
func TestMarkdownParagraphClass(t *testing.T) {
	blocks := mdBlocks("hello\n\n## h\n\n- item\n\n  more", mdCtx{pClass: "page-intro", media: testMedia})
	got := make([]string, len(blocks))
	for i, b := range blocks {
		got[i] = string(b)
	}
	want := []string{
		`<p class="page-intro">hello</p>`,
		"<h2>h</h2>",
		"<ul>\n<li>\n<p>item</p>\n<p>more</p>\n</li>\n</ul>",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("pClass blocks =\n %q\nwant\n %q", got, want)
	}
}

// Property 1: raw HTML is not a syntax. A config field is written by whoever
// runs the site, but it is still data, and no data may reach the page as
// markup: that is what keeps a config file from breaking the layout or the
// CSP. goldmark would either render it (WithUnsafe) or swap it for an
// "omitted" comment; cairn parses none of it, so it stays text and escapes.
func TestMarkdownNeverEmitsRawHTML(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<b>raw</b>", "<p>&lt;b&gt;raw&lt;/b&gt;</p>"},
		{"<script>alert(1)</script>", "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>"},
		{"a <b>bold</b> word", "<p>a &lt;b&gt;bold&lt;/b&gt; word</p>"},
		{`<img src=x onerror="alert(1)">`, `<p>&lt;img src=x onerror=&quot;alert(1)&quot;&gt;</p>`},
		{"<div>\nblock\n</div>", "<p>&lt;div&gt;\nblock\n&lt;/div&gt;</p>"},
		{"<!-- comment -->", "<p>&lt;!-- comment --&gt;</p>"},
		{"<style>body{display:none}</style>", "<p>&lt;style&gt;body{display:none}&lt;/style&gt;</p>"},
	} {
		if got := mdOne(t, tc.in, mdCtx{media: testMedia}); got != tc.want {
			t.Errorf("raw HTML %q = %q, want %q", tc.in, got, tc.want)
		}
	}
	// Nothing anywhere in the output may be a tag cairn did not write itself,
	// so the check is also made across a whole field rather than one block at
	// a time.
	for _, b := range mdBlocks("<b>x</b>\n\n<script>y</script>\n\n> <i>z</i>", mdCtx{media: testMedia}) {
		if strings.Contains(string(b), "<b>") || strings.Contains(string(b), "<script") ||
			strings.Contains(string(b), "<i>") || strings.Contains(string(b), "<!--") {
			t.Errorf("block leaked raw HTML: %q", b)
		}
	}
}

// Property 2: link schemes are an allow-list. goldmark ships a deny-list
// (javascript:, vbscript:, file:, non-image data:) and lets everything else
// become a live link, so this rule is the only thing between a config field
// and an href the browser will act on.
func TestMarkdownLinkSchemeAllowList(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// Refused: the words survive, the anchor does not.
		{"[x](javascript:alert(1))", "x"},
		{"[x](JavaScript:alert(1))", "x"},
		{"[x](vbscript:msgbox(1))", "x"},
		{"[x](data:text/html,alert)", "x"},
		{"[x](data:image/png;base64,AAA)", "x"},
		{"[x](file:///etc/passwd)", "x"},
		{"[x](ftp://x.org/a)", "x"},
		{"[x](tel:+33123456789)", "x"},
		{"[x](chrome://settings)", "x"},
		// goldmark decodes entity references in a destination before this
		// runs, so an obfuscated scheme is checked as the browser sees it.
		{"[x](&#106;avascript:alert&#40;1&#41;)", "x"},
		// An autolink is a link too, and "javascript:" is a valid one.
		{"<javascript:alert(1)>", "javascript:alert(1)"},
		{"<vbscript:msgbox(1)>", "vbscript:msgbox(1)"},
		// Allowed: what a directory page has a reason to point at.
		{"[x](https://x.org/a)", `<a href="https://x.org/a">x</a>`},
		{"[x](http://x.org/a)", `<a href="http://x.org/a">x</a>`},
		{"[x](mailto:a@b.org)", `<a href="mailto:a@b.org">x</a>`},
		{"[x](/fr/legal/)", `<a href="/fr/legal/">x</a>`},
		{"[x](#anchor)", `<a href="#anchor">x</a>`},
		{"[x](../up/)", `<a href="../up/">x</a>`},
		{"<https://x.org>", `<a href="https://x.org">https://x.org</a>`},
		{"<a@b.org>", `<a href="mailto:a@b.org">a@b.org</a>`},
	} {
		if got := mdPara(t, tc.in); got != tc.want {
			t.Errorf("scheme %q = %q, want %q", tc.in, got, tc.want)
		}
	}
	// An image src is a URL the browser fetches, so it answers to the same
	// rule; refused, the caption is all that is left.
	if got := mdOne(t, "![cap](javascript:alert(1))", mdCtx{}); got != "<p>cap</p>" {
		t.Errorf("refused image src = %q, want <p>cap</p>", got)
	}
}

// Property 3: an image alone in a block is a figure, and it carries the width
// and height the resolver knows. Those two attributes are why a page does not
// jump as its screenshots arrive, and they are the first thing a rewrite of
// this file would drop without noticing.
func TestMarkdownFigureShape(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"![The plan](plan.png)", `<figure class="shot"><img src="/media/plan.png" alt="" loading="lazy" width="800" height="600"><figcaption>The plan</figcaption></figure>`},
		// No caption, no figcaption.
		{"![](plan.png)", `<figure class="shot"><img src="/media/plan.png" alt="" loading="lazy" width="800" height="600"></figure>`},
		// Unknown size, so no width and no height rather than a guess.
		{"![shot](other.png)", `<figure class="shot"><img src="/media/other.png" alt="" loading="lazy"><figcaption>shot</figcaption></figure>`},
		// The caption keeps its inline markup.
		{"![a **bold** cap](plan.png)", `<figure class="shot"><img src="/media/plan.png" alt="" loading="lazy" width="800" height="600"><figcaption>a <strong>bold</strong> cap</figcaption></figure>`},
		// An image inside a sentence is a plain inline <img>, not a figure and
		// no longer the link-with-a-stray-! the old parser produced.
		{"text with ![inline](plan.png) in it", `<p>text with <img src="/media/plan.png" alt="inline" loading="lazy" width="800" height="600"> in it</p>`},
		// Two in one block are both inline: a figure is a block on its own.
		{"![a](plan.png) ![b](plan.png)", `<p><img src="/media/plan.png" alt="a" loading="lazy" width="800" height="600"> <img src="/media/plan.png" alt="b" loading="lazy" width="800" height="600"></p>`},
	} {
		if got := mdOne(t, tc.in, mdCtx{media: testMedia}); got != tc.want {
			t.Errorf("figure %q =\n %q\nwant\n %q", tc.in, got, tc.want)
		}
	}
}

// The syntax the old subset rendered as literal text, or as wrong markup. Each
// line here was a bug report waiting to happen: an operator writes ordinary
// markdown in a config field and reads it back verbatim on the page.
func TestMarkdownFullSyntax(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// A single # demotes: the page owns its <h1>. Pinned in full by
		// TestMarkdownDemotesATopHeading.
		{"# Title", "<h2>Title</h2>"},
		{"### Deep", "<h3>Deep</h3>"},
		{"1. one\n2. two", "<ol>\n<li>one</li>\n<li>two</li>\n</ol>"},
		{"- a\n  - b", "<ul>\n<li>a\n<ul>\n<li>b</li>\n</ul>\n</li>\n</ul>"},
		{"> quoted", "<blockquote>\n<p>quoted</p>\n</blockquote>"},
		{"```go\nfmt.Println()\n```", "<pre><code class=\"language-go\">fmt.Println()\n</code></pre>"},
		{"    indented", "<pre><code>indented\n</code></pre>"},
		{"---", "<hr>"},
		{"| a | b |\n| - | - |\n| 1 | 2 |", "<div class=\"table-wrap\" tabindex=\"0\"><table><thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>1</td>\n<td>2</td>\n</tr>\n</tbody>\n</table></div>"},
		{"~~gone~~", "<p><del>gone</del></p>"},
		{"_em_ and __strong__", "<p><em>em</em> and <strong>strong</strong></p>"},
		{"visit https://x.org now", `<p>visit <a href="https://x.org">https://x.org</a> now</p>`},
		{"mail a@b.org", `<p>mail <a href="mailto:a@b.org">a@b.org</a></p>`},
		{"line one  \nline two", "<p>line one<br>\nline two</p>"},
		{`\*not em\*`, "<p>*not em*</p>"},
		{"- [ ] todo", "<ul>\n<li><input disabled=\"\" type=\"checkbox\"> todo</li>\n</ul>"},
		{"3. three\n4. four", "<ol start=\"3\">\n<li>three</li>\n<li>four</li>\n</ol>"},
	} {
		if got := mdOne(t, tc.in, mdCtx{media: testMedia}); got != tc.want {
			t.Errorf("syntax %q =\n %q\nwant\n %q", tc.in, got, tc.want)
		}
	}
}

// cairn's CSP is style-src 'self' plus one hash for the accent block, so an
// inline style attribute anywhere in a rendered field is dropped by the
// browser and reported as a violation. goldmark aligns table columns with
// style="text-align:…" unless told otherwise, which is the one place markdown
// can produce one.
func TestMarkdownEmitsNoInlineStyle(t *testing.T) {
	aligned := "| a | b | c |\n|:--|:-:|--:|\n| 1 | 2 | 3 |"
	want := "<div class=\"table-wrap\" tabindex=\"0\"><table><thead>\n<tr>\n<th align=\"left\">a</th>\n<th align=\"center\">b</th>\n<th align=\"right\">c</th>\n</tr>\n</thead>\n" +
		"<tbody>\n<tr>\n<td align=\"left\">1</td>\n<td align=\"center\">2</td>\n<td align=\"right\">3</td>\n</tr>\n</tbody>\n</table></div>"
	if got := mdOne(t, aligned, mdCtx{media: testMedia}); got != want {
		t.Errorf("aligned table =\n %q\nwant\n %q", got, want)
	}
	for _, src := range []string{aligned, "# h", "> q", "- [x] done", "![c](plan.png)", "[a](https://x.org)"} {
		for _, b := range mdBlocks(src, mdCtx{media: testMedia}) {
			if strings.Contains(string(b), "style=") {
				t.Errorf("inline style in %q: %q", src, b)
			}
		}
	}
}

func TestMarkdownEndToEnd(t *testing.T) {
	dir := testutil.WriteFiles(t, map[string]string{
		"site.yaml": "locales: [fr]\nabout: |\n  Un **annuaire**, [contact](mailto:a@b.org).\n" +
			"pages:\n  - id: legal\n    title: Legal\n    body: |\n      ## Editeur\n\n      - Prénom Nom\n      - Adresse\n",
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: A\n  details: |\n    Voir la [doc](https://ex.org).\n",
	})
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	home := string(m.Pages["fr"].HTML)
	if !strings.Contains(home, "<strong>annuaire</strong>") || !strings.Contains(home, `<a href="mailto:a@b.org">contact</a>`) {
		t.Errorf("about markdown not rendered:\n%s", home)
	}
	legal := string(m.Pages["fr/legal"].HTML)
	if !strings.Contains(legal, "<h2>Editeur</h2>") || !strings.Contains(legal, "<li>Adresse</li>") {
		t.Errorf("page markdown not rendered:\n%s", legal)
	}
	detail := string(m.Pages["fr/a"].HTML)
	if !strings.Contains(detail, `<a href="https://ex.org">doc</a>`) {
		t.Errorf("details markdown not rendered:\n%s", detail)
	}
}

// A link the writer got wrong must come out as visible text, never as broken
// markup and never as a live link to something unintended.
func TestMalformedLinksStayText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"[no closing paren](https://x.org", `[no closing paren](<a href="https://x.org">https://x.org</a>`},
		{"[no target]", "[no target]"},
		{"[empty]()", `<a href="">empty</a>`},
		{"[js](javascript:alert(1))", "js"},
		{"[ok](https://x.org)", `<a href="https://x.org">ok</a>`},
		{"[relative](/legal/)", `<a href="/legal/">relative</a>`},
		{"[mail](mailto:a@b.org)", `<a href="mailto:a@b.org">mail</a>`},
	} {
		if got := mdPara(t, c.in); got != c.want {
			t.Errorf("malformed %q = %q, want %q", c.in, got, c.want)
		}
	}
}

// A bracket earlier in the paragraph used to be swallowed into the next link's
// text, because the parser looked for the first "](" anywhere ahead instead of
// closing at the first "]". There was no way to write a literal [word] in a
// paragraph that also held a link.
func TestMarkdownLinkStopsAtItsOwnBracket(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"See [docs] and [here](https://x.org) too",
			`See [docs] and <a href="https://x.org">here</a> too`},
		{"array[0] then [link](https://x.org)",
			`array[0] then <a href="https://x.org">link</a>`},
		{"[not a link] plain text", "[not a link] plain text"},
		{"[a](https://x.org) and [b](https://y.org)",
			`<a href="https://x.org">a</a> and <a href="https://y.org">b</a>`},
	} {
		if got := mdPara(t, c.in); got != c.want {
			t.Errorf("bracket %q\n got %s\nwant %s", c.in, got, c.want)
		}
	}
}

// mdText feeds the meta description, which is plain text inside an attribute:
// no tags, no entities, and no newline where the markup had one.
func TestMarkdownText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"**Contact** : [me](mailto:a@b.org) & you", "Contact : me & you"},
		{"multi\nline\nparagraph", "multi line paragraph"},
		{"- one\n- two", "one two"},
		{"## Heading", "Heading"},
		{"a `code` span", "a code span"},
		{"[js](javascript:alert(1))", "js"},
	} {
		if got := mdText(c.in); got != c.want {
			t.Errorf("mdText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The table scrolls from its wrapper, not from display:block on itself.
// Chrome and Safari drop the table role from the accessibility tree when a
// <table> is display:block, so the cells stop being announced as a table, and
// a scroll container that is not focusable cannot be reached without a
// pointer. Both are invisible on a wide screen with a mouse, which is why this
// is pinned rather than left to the stylesheet.
func TestMarkdownTableScrollsFromAFocusableWrapper(t *testing.T) {
	got := mdOne(t, "| a | b |\n| - | - |\n| 1 | 2 |", mdCtx{media: testMedia})
	if !strings.HasPrefix(got, `<div class="table-wrap" tabindex="0"><table>`) {
		t.Errorf("table is not inside a focusable wrapper:\n%s", got)
	}
	if !strings.HasSuffix(got, "</table></div>") {
		t.Errorf("wrapper is not closed around the table:\n%s", got)
	}
	if strings.Contains(got, "<table ") {
		t.Errorf("the table carries attributes of its own, so the wrapper is not doing the work:\n%s", got)
	}
}

// A level-one heading in a field becomes a level two. Every page already has
// its own <h1>, and a second one leaves a screen reader user navigating by
// level with two competing tops. Both spellings demote, and nothing below
// level one moves, so no page written against the old parser changes shape.
func TestMarkdownDemotesATopHeading(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"# One", "<h2>One</h2>"},
		{"Setext\n======", "<h2>Setext</h2>"},
		{"## Two", "<h2>Two</h2>"},
		{"### Three", "<h3>Three</h3>"},
		{"###### Six", "<h6>Six</h6>"},
	} {
		if got := mdOne(t, c.in, mdCtx{media: testMedia}); got != c.want {
			t.Errorf("heading %q = %q, want %q", c.in, got, c.want)
		}
	}
	// And the operator can still be told what they wrote, which is what -check
	// reads: the demotion must not hide the source from the check that
	// explains it.
	for _, c := range []struct {
		in   string
		want bool
	}{{"# One", true}, {"Setext\n======", true}, {"## Two", false}, {"no heading", false}, {"", false}} {
		if got := HasTopHeading(c.in); got != c.want {
			t.Errorf("HasTopHeading(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
