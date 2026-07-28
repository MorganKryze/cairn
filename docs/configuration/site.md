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
| `favicon`      | cairn's   | The tab icon: a URL or an `/assets` path. See [the icon set](#the-icon-set) for what replacing it costs. |
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
| `status`       | none      | Live status pills fed by your Gatus (`status.gatus`, `status.page`, `status.interval`, `status.linked`); see [Status page](../recipes/gatus.md). |

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

## The icon set

Out of the box cairn serves a complete set, all generated from one drawing so
none of it can drift:

| File | Where it shows up |
| ---- | ----------------- |
| `favicon.svg` | The browser tab, at any size and on any tab colour. |
| `favicon.ico` | 16, 32 and 48 pixels, for the feed readers and link previewers that fetch `/favicon.ico` and never read the html. |
| `touch-icon.png` | 180 pixels: an iPhone or iPad home screen. |
| `icon-192.png`, `icon-512.png` | Android. Chromium offers "install this site" only when the manifest carries both, and cairn's are declared `maskable`, so Android's round crop takes nothing off. |

Nothing here is fetched from anywhere: like every other asset, the icons are
compiled into the binary and served from it.

### Using your own

Set `favicon` and it becomes the whole set. What cairn then tells browsers
about it depends on what it can actually establish, and it never guesses:

| What you set | What the manifest says |
| ------------ | ---------------------- |
| An **svg**, anywhere | `sizes: any`. An svg is every size, so yours serves the home screen exactly the way cairn's does. |
| A **raster in `/assets`** | Its real width and height, measured when the config loads. |
| A **raster behind a URL** | The file, with no size. Measuring it would mean an outbound request, and cairn makes none. |

An entry without a size is still a valid manifest entry, and browsers still
use the icon. What it costs is the install prompt: Chromium offers "install
this site" only when it sees both a 192 and a 512.

To get that with your own artwork, name the files yourself:

```yaml
icons:
  - { src: /assets/brand-192.png, sizes: 192x192 }
  - { src: /assets/brand-512.png, sizes: 512x512, purpose: any maskable }
```

`icons` replaces everything derived from `favicon`, which keeps serving the
browser tab. That includes iOS, which ignores the manifest and reads the
`apple-touch-icon` link alone: cairn points it at the largest entry in your
list, so an iPhone home screen shows your artwork and not cairn's.

`sizes` is `WIDTHxHEIGHT`, or `any` for an svg. `purpose` is
optional and accepts `any`, `maskable` and `monochrome`, combined if you like.
A **maskable** icon must keep its content inside the middle 80% of the square,
because Android crops away the rest.

cairn does not resize images: doing it would mean shipping a scaler for one
rarely used path, and a badly resampled logo is worse than the one you drew.
Anything wrong in this list is a config error that names the entry, so a typo
stops at startup rather than reaching a phone.

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
