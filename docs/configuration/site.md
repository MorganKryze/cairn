# Site

`site.yaml` describes everything around the cards. The file is optional, and
so is every key in it.

## Keys

| Key            | Default   | What it is                                        |
| -------------- | --------- | ------------------------------------------------- |
| `title`        | `cairn`   | Site name: header, `<title>`. Translatable like every text key. |
| `tagline`      | empty     | One sentence about the site, translatable. Opens the home page and feeds the meta description search engines and link previews show. |
| `url`          | none      | Public base URL, e.g. `https://tools.example.org`. Enables canonical and hreflang links per page, absolute sitemap URLs and the social preview image; recommended for search engines. |
| `logo`         | none      | Image in the header: a URL or an [`/assets` path](../recipes/icons.md#your-own-files). A raster logo (png, jpg…) also becomes the link-preview image when `url` is set. |
| `favicon`      | cairn's   | The tab icon: a URL or an `/assets` path.         |
| `index`        | `true`    | `index: false` asks search engines to stay away: `robots.txt` disallows everything, every page carries `noindex`, the sitemap turns off. For directories meant for ten people, not the whole web. |
| `locales`      | `[en]`    | Languages served. First entry is the default and the fallback; see [Languages](i18n.md). |
| `theme.accent` | `#247b7b` | Hex color for links, focus rings, buttons; see [Theming](theming.md). |
| `about`        | empty     | A welcome note under the hero, translatable; visitors can dismiss it (cookie, one year). Long text: paragraphs and the [markdown subset](text.md). |
| `links`        | `[]`      | Header navigation links, with optional icons; see below. |
| `footer`       | `[]`      | Links at the bottom of every page.                |
| `pages`        | `[]`      | Pages cairn serves itself (legal notice, privacy…); see below. Bodies accept the [markdown subset](text.md). |
| `credit`       | `true`    | The small "powered by cairn" in the footer. `credit: false` removes it. |
| `show_version` | `false`   | Prints the running cairn version beside that credit: the release number for a tagged build, the commit for a build off `main`. Handy when you report a bug, or run several instances. |
| `strings`      | built-ins | UI text overrides; see [Languages](i18n.md#ui-strings). |
| `status`       | none      | Live status pills fed by your Gatus (`status.gatus`, `status.page`, `status.interval`); see [Status page](../recipes/gatus.md). |

## Full example

```yaml
title: Libre Internet
tagline:
  fr: Des outils libres, simples, pour tout le monde.
  en: Free, simple tools for everyone.
logo: /assets/logo.png
locales: [fr, en]
theme:
  accent: "#247b7b"
about:
  fr: |
    Bienvenue ! Voici les services que j'héberge pour vous.
  en: |
    Welcome! These are the services I host for you.
links:
  - label: Wiki
    url: https://wiki.example.org
    icon: book
  - label: GitHub
    url: https://github.com/you
    icon: github
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
  - label: Contact
    url: mailto:admin@example.org
pages:
  - id: legal
    title: { fr: Mentions légales, en: Legal notice }
    body:
      fr: |
        Éditeur : Prénom Nom, contact@exemple.org

        Hébergement : ce site est auto-hébergé par son éditeur.
      en: |
        Publisher: First Last, contact@example.org

        Hosting: this site is self-hosted by its publisher.
```

A typo in a key name is an error, not a silent no-op: cairn names the file
and the unknown key in the log.

## Header links

The header has two rows: your identity (logo, title, languages), then a
navigation row holding the `links` on the left and the search on the right.

Each link takes an optional `icon`. Name one of the built-in glyphs, drawn
inline so they cost no external request:

`book` · `chat` · `github` · `globe` · `heart` · `home` · `key` · `mail` ·
`portfolio` · `rss` · `status` · `user`

Or pass a URL or an `/assets` path for your own image. An unknown glyph name
is a config error that lists the valid ones.

## The welcome note

`about` renders as a framed note between the hero and the categories: your
place to tell visitors where they landed, in your words and languages.
Visitors can dismiss it with the button in its corner; a cookie remembers
the choice for a year, and without JavaScript the note simply stays. The
button label lives in the `strings` table as `about.dismiss`.

## Hosted pages

Self-hosters rarely have another place to put a legal notice or a privacy
policy, so cairn serves these pages itself. Each entry in `pages` becomes
`/{locale}/{id}/`, rendered in the site layout: the translatable `title` as
the heading, the translatable `body` as plain-text paragraphs (blank line =
new paragraph, no markup). Every page is linked automatically at the end of
the footer, after your `footer` entries, in declaration order.

For structured pages, add `sections`: titled blocks rendered after the
body. With sections present, the body becomes an optional intro; without
them, it carries the whole page. Either way a page needs at least one of
the two. The demo ships a filled-in legal notice and privacy policy built
this way; copy them and replace the placeholders.

```yaml
pages:
  - id: legal
    title: { fr: Mentions légales, en: Legal notice }
    sections:
      - title: { fr: Éditeur, en: Publisher }
        body: { fr: "Prénom Nom, contact@exemple.org.", en: "…" }
      - title: { fr: Hébergement, en: Hosting }
        body: { fr: "Ce site est auto-hébergé par son éditeur.", en: "…" }
```

Page ids share the URL namespace with service ids; a collision is a config
error that names both sides.

Next: [Writing text](text.md)
