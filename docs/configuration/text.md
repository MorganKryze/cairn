# Writing text

Three fields hold long text: the welcome note (`about`), the hosted pages
(`pages`, body and sections) and the service detail pages (`details`). All
three share the same rules, described here. Short fields (`name`, `desc`,
`tagline`, image captions, `strings`) stay plain text on purpose: they live
in places where formatting would be noise.

## Paragraphs

A blank line starts a new paragraph. In YAML, use a literal block (`|`) to
keep your line breaks:

```yaml
about: |
  First paragraph.

  Second paragraph.
```

## CommonMark, minus raw HTML

Long text is [CommonMark](https://commonmark.org) with the GitHub extensions:
the markdown you already write in a README. Headings at every level, bulleted
and numbered lists, nesting, emphasis with either `*stars*` or `_underscores_`,
`inline code` and fenced code blocks, quotations, tables, horizontal rules,
`~~strikethrough~~`, task lists, link titles, backslash escapes. An address you
write bare, `https://example.org` or `you@example.org`, is linked for you.

That list is the least useful part of this page, because you already knew it.
What is worth reading is the three places cairn does not behave like the
preview in your editor, and the two behaviours that catch people out.

### Raw HTML is text, not markup

cairn parses no HTML at all. `<b>bold</b>` in a config field reaches the page
as the eleven characters you typed, and a `<script>` tag shows up on screen
reading `<script>` and does nothing.

That is the rule everything else leans on. A config file is written by whoever
runs the site, but it is still data arriving from outside the program, and no
data becomes markup on the way to the page. A stray angle bracket cannot break
the layout, and a snippet pasted from somewhere else cannot break the
[security headers](../../SECURITY.md#scope-worth-knowing).

Refusing is not the same as deleting, and cairn does not delete. Markdown
renderers commonly replace raw HTML with an `<!-- raw HTML omitted -->`
comment, which loses the sentence you wrote without telling you. Here your
words are always on the page, exactly as typed.

### A link cairn cannot write keeps its words

`[label](url)` accepts `https://`, `http://`, `mailto:` and anything with no
scheme at all: `/legal/`, `../up/`, `#section`. Every other scheme keeps its
words and loses its link, `tel:` and `ftp:` as surely as `javascript:` and
`data:`:

| You write                         | Visitors see                        |
| --------------------------------- | ----------------------------------- |
| `[the docs](https://example.org)` | **the docs**, linked out            |
| `[write me](mailto:you@here.org)` | **write me**, opening a mail window |
| `[our terms](/en/legal/)`         | **our terms**, linked to your page  |
| `[call us](tel:+33123456789)`     | **call us**, with no link on it     |

That is the same list [header and footer links](site.md#links-a-browser-will-not-follow)
are held to, and for the same reason: `html/template` writes those into an
`href` and replaces every other scheme with a placeholder that goes nowhere,
so a link cairn cannot write is a link that would render dead. It applies to
`<https://example.org>` autolinks and to image sources too.

What differs is how loudly you hear about it. A `tel:` in `footer` is a config
error that names the file and the line, because that link is the whole entry
and a dead one is worthless. A `tel:` in prose is silent: the sentence still
reads, and refusing to serve a legal notice over one link inside it is not a
trade worth making. Nothing warns you, so if you meant a phone number, write
it out and link to a page instead.

### An image alone in a block is a framed figure

An image on its own, with blank lines around it, becomes a framed figure with
its caption underneath:

```yaml
body: |
  ![The albums view](albums.png)
```

The same image written inside a sentence stays inline, at whatever size the
file is:

```yaml
body: |
  Look for the ![gear icon](gear.png) in the corner.
```

Either way, images resolve exactly like
[service preview images](services.md#preview-images): a bare name is a file in
your `media/` folder, a URL or an absolute path passes through. When cairn can
open the file it also writes the width and height into the page, so nothing
shifts under the reader while the image loads.

They are checked differently from `images:` entries, though, and it is worth
knowing which way round. A missing `images:` entry stops the load and names
itself; a missing one here only earns a `cairn -check` warning, because prose
is not worth refusing a whole site over:

```console
warning: page "legal" body shows "seal.png", which is not there: the page renders a broken image (expected a file at ./config/media/seal.png)
```

## Two things that catch people out

### A single `#` comes out as `##`

Every page cairn renders already has one `<h1>`: the site title on the home
page, the page or service title everywhere else. A second one leaves a screen
reader with two competing answers to "what is this page about", and someone
navigating by heading level with no way to tell which is the page.

So cairn demotes it. `# Title` in your text renders `<h2>`, and every level
below is untouched:

| You write   | You get |
| ----------- | ------- |
| `# Title`   | `<h2>`  |
| `## Title`  | `<h2>`  |
| `### Title` | `<h3>`  |

The two top spellings therefore produce the same thing, which is the one part
worth knowing, and `cairn -check` says so rather than leaving you to discover
it:

```console
warning: page "legal" body opens a heading with a single #: cairn renders it as ## because the page already has its own top heading, so # and ## come out the same (write ## and the output is unchanged)
```

Write `##` and the output is identical. Nothing is lost either way.

There is also a way to write a top heading without typing `#` at all:

```yaml
body: |
  A perfectly ordinary line.
  ===
```

A line of `=` directly under text is an older markdown spelling of `<h1>`, and
it demotes the same way. A line of `-` is the same spelling of `<h2>`, which is
why a `---` rule needs a blank line above it: with text immediately above,
`---` is a heading and not a rule.

### `[^1]` is not a footnote

Footnotes are not part of CommonMark, and cairn adds no extension for them. The
marker never becomes a superscript and the note never moves to the bottom of
the page. What happens instead depends on the note, and neither outcome is what
you meant. This one:

```yaml
body: |
  Hosted in France[^1].

  [^1]: In a rack in the spare room.
```

shows `Hosted in France[^1].` with the brackets visible, then the note as an
ordinary paragraph right underneath it. A one-word note is worse:

```yaml
body: |
  Hosted in France[^1].

  [^1]: attic
```

CommonMark reads that second line as the definition of a link named `^1`. The
line vanishes from the page, and `[^1]` in the first line becomes a live link
to a page called `attic` that does not exist.

Put the aside in the sentence, in a paragraph of its own, or in its own
`sections` entry.

## A full example

```yaml
pages:
  - id: legal
    title: { fr: Mentions légales, en: Legal notice }
    body:
      fr: |
        ## Éditeur

        Ce site est édité par **Prénom Nom**.
        Contact : [prenom@exemple.org](mailto:prenom@exemple.org).

        - hébergé à la maison
        - aucune donnée collectée

        ## Hébergement

        | Ressource  | Valeur                 |
        | ---------- | ---------------------- |
        | Serveur    | une machine au grenier |
        | Sauvegarde | un disque, hors ligne  |
```

Next: [Theming](theming.md)
