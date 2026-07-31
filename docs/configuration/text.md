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

## The markdown subset

Long text accepts a small, deliberate subset of markdown:

| You write                                  | Visitors see                                    |
| ------------------------------------------ | ----------------------------------------------- |
| `[label](url)`                             | a link; `https:`, `mailto:` and paths work      |
| `**words**`                                | **bold**                                        |
| `*words*`                                  | _italic_                                        |
| `` `words` ``                              | `inline code`, good for cookie or service names |
| `- item` (one per line)                    | a bulleted list                                 |
| `## Title` (alone, blank lines around)     | a small section heading                         |
| `![caption](file.png)` (alone on its line) | a framed image with its caption                 |

Images resolve exactly like [service preview images](services.md#preview-images):
a bare name is a file in your `media/` folder, a URL or an absolute path
passes through.

A full example, in a hosted page:

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
```

## What is left out, on purpose

No raw HTML, no tables, no nested or numbered lists, no headings deeper
than `##`, and no link schemes beyond `http(s)`, `mailto` and plain paths.
Anything the subset does not recognize renders exactly as you typed it, so
a stray asterisk cannot break a page. Text can never inject markup: what
you write is escaped, and the
[security headers](../../SECURITY.md#scope-worth-knowing) stay strict.

Next: [Theming](theming.md)
