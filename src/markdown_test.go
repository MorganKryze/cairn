package main

import (
	"strings"
	"testing"
)

func TestMarkdownInline(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"plain text", "plain text"},
		{"**bold** and *italic*", "<strong>bold</strong> and <em>italic</em>"},
		{"a `code` span", "a <code>code</code> span"},
		{"[write me](mailto:a@b.org)", `<a href="mailto:a@b.org">write me</a>`},
		{"[docs](https://example.org/d)", `<a href="https://example.org/d">docs</a>`},
		{"[legal](/fr/legal/)", `<a href="/fr/legal/">legal</a>`},
		{"[bad](javascript:alert(1))", "[bad](javascript:alert(1))"},
		{"**[bold link](https://x.org)**", `<strong><a href="https://x.org">bold link</a></strong>`},
		{"5 < 6 & 7 > 2", "5 &lt; 6 &amp; 7 &gt; 2"},
		{"`<script>`", "<code>&lt;script&gt;</code>"},
		{"a lone * star", "a lone * star"},
		{"unclosed **bold", "unclosed **bold"},
		{"unclosed [link(https://x", "unclosed [link(https://x"},
	} {
		if got := mdInline(tc.in); got != tc.want {
			t.Errorf("mdInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMarkdownBlocks(t *testing.T) {
	media := func(src string) (string, int, int) {
		if src == "plan.png" {
			return "/media/plan.png", 800, 600
		}
		return mediaURL(src), 0, 0
	}
	src := "## Contact\n\nWrite to [me](mailto:a@b.org).\n\n- one\n- two\n\n![The plan](plan.png)"
	blocks := mdBlocks(src, mdCtx{media: media})
	if len(blocks) != 4 {
		t.Fatalf("blocks = %d, want 4", len(blocks))
	}
	for i, want := range []string{
		"<h2>Contact</h2>",
		`<p>Write to <a href="mailto:a@b.org">me</a>.</p>`,
		"<ul><li>one</li><li>two</li></ul>",
		`<figure class="shot"><img src="/media/plan.png" alt="" loading="lazy" width="800" height="600"><figcaption>The plan</figcaption></figure>`,
	} {
		if string(blocks[i]) != want {
			t.Errorf("block %d = %q, want %q", i, blocks[i], want)
		}
	}
	if got := string(mdBlocks("hello", mdCtx{pClass: "page-intro", media: media})[0]); got != `<p class="page-intro">hello</p>` {
		t.Errorf("pClass block = %q", got)
	}
	if got := mdText("**Contact** : [me](mailto:a@b.org) & you"); got != "Contact : me & you" {
		t.Errorf("mdText = %q", got)
	}
}

func TestMarkdownEndToEnd(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"site.yaml": "locales: [fr]\nabout: |\n  Un **annuaire**, [contact](mailto:a@b.org).\n" +
			"pages:\n  - id: legal\n    title: Legal\n    body: |\n      ## Editeur\n\n      - Prénom Nom\n      - a@b.org\n",
		"services.yaml": "- id: a\n  url: https://a.example.org\n  name: A\n  details: |\n    Voir la [doc](https://ex.org).\n",
	})
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	home := string(m.Pages["fr"].HTML)
	if !strings.Contains(home, "<strong>annuaire</strong>") || !strings.Contains(home, `<a href="mailto:a@b.org">contact</a>`) {
		t.Errorf("about markdown not rendered:\n%s", home)
	}
	legal := string(m.Pages["fr/legal"].HTML)
	if !strings.Contains(legal, "<h2>Editeur</h2>") || !strings.Contains(legal, "<li>a@b.org</li>") {
		t.Errorf("page markdown not rendered:\n%s", legal)
	}
	detail := string(m.Pages["fr/a"].HTML)
	if !strings.Contains(detail, `<a href="https://ex.org">doc</a>`) {
		t.Errorf("details markdown not rendered:\n%s", detail)
	}
}
