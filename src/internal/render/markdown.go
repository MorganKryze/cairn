package render

// cairn's markdown is a deliberately tiny subset for the long text fields
// (about, pages, details): [links](…), **bold**, *italic*, `code`, "- "
// lists, "## " headings and "![caption](src)" figures. Everything else is
// text, HTML-escaped; there is no raw HTML and no full CommonMark on
// purpose, so a config file can never break the page or the CSP.

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
)

// mdCtx carries the per-render options: the css class of plain paragraphs
// and the image resolver (src -> url, width, height; 0x0 when unknown).
type mdCtx struct {
	pClass string
	media  func(string) (string, int, int)
}

var mdImgRe = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)\s]+)\)$`)

func mdBlocks(src string, ctx mdCtx) []template.HTML {
	var out []template.HTML
	for _, block := range strings.Split(src, "\n\n") {
		if block = strings.TrimSpace(block); block != "" {
			out = append(out, mdBlock(block, ctx))
		}
	}
	return out
}

func mdBlock(block string, ctx mdCtx) template.HTML {
	if t, ok := strings.CutPrefix(block, "## "); ok && !strings.Contains(block, "\n") {
		return template.HTML("<h2>" + mdInline(t) + "</h2>")
	}
	if m := mdImgRe.FindStringSubmatch(block); m != nil && ctx.media != nil {
		url, w, h := ctx.media(m[2])
		img := `<img src="` + html.EscapeString(url) + `" alt="" loading="lazy"`
		if w > 0 {
			img += fmt.Sprintf(` width="%d" height="%d"`, w, h)
		}
		caption := ""
		if m[1] != "" {
			caption = "<figcaption>" + mdInline(m[1]) + "</figcaption>"
		}
		return template.HTML(`<figure class="shot">` + img + ">" + caption + "</figure>")
	}
	if strings.HasPrefix(block, "- ") {
		var b strings.Builder
		b.WriteString("<ul>")
		for _, line := range strings.Split(block, "\n") {
			b.WriteString("<li>" + mdInline(strings.TrimPrefix(strings.TrimSpace(line), "- ")) + "</li>")
		}
		b.WriteString("</ul>")
		return template.HTML(b.String())
	}
	p := "<p>"
	if ctx.pClass != "" {
		p = `<p class="` + ctx.pClass + `">`
	}
	return template.HTML(p + mdInline(strings.ReplaceAll(block, "\n", " ")) + "</p>")
}

func mdInline(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		i := strings.IndexAny(s, "`*[")
		if i < 0 {
			b.WriteString(html.EscapeString(s))
			break
		}
		b.WriteString(html.EscapeString(s[:i]))
		s = s[i:]
		switch {
		case s[0] == '`':
			if j := strings.IndexByte(s[1:], '`'); j >= 0 {
				b.WriteString("<code>" + html.EscapeString(s[1:1+j]) + "</code>")
				s = s[j+2:]
				continue
			}
		case strings.HasPrefix(s, "**"):
			if j := strings.Index(s[2:], "**"); j > 0 {
				b.WriteString("<strong>" + mdInline(s[2:2+j]) + "</strong>")
				s = s[j+4:]
				continue
			}
		case s[0] == '*':
			if j := strings.IndexByte(s[1:], '*'); j > 0 {
				b.WriteString("<em>" + mdInline(s[1:1+j]) + "</em>")
				s = s[j+2:]
				continue
			}
		case s[0] == '[':
			if txt, url, n := mdLink(s); n > 0 && mdSafeURL(url) {
				b.WriteString(`<a href="` + html.EscapeString(url) + `">` + mdInline(txt) + "</a>")
				s = s[n:]
				continue
			}
		}
		b.WriteString(html.EscapeString(s[:1]))
		s = s[1:]
	}
	return b.String()
}

func mdLink(s string) (txt, url string, n int) {
	sep := strings.Index(s, "](")
	if sep < 0 {
		return "", "", 0
	}
	end := strings.IndexByte(s[sep+2:], ')')
	if end < 0 {
		return "", "", 0
	}
	return s[1:sep], s[sep+2 : sep+2+end], sep + 2 + end + 1
}

// mdSafeURL allows the schemes a directory page has a reason to link:
// anything scheme-less (paths, anchors, relative), http(s) and mailto.
func mdSafeURL(u string) bool {
	for _, p := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return !strings.Contains(u, ":")
}

var mdTagRe = regexp.MustCompile(`<[^>]*>`)

// mdText renders a block to plain text, for meta descriptions.
func mdText(s string) string {
	return html.UnescapeString(mdTagRe.ReplaceAllString(mdInline(strings.ReplaceAll(s, "\n", " ")), ""))
}
