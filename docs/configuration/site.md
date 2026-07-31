# Site

`site.yaml` describes everything around the cards. The file is optional, and
so is every key in it.

## Keys

| Key            | Default   | What it is                                                                                                                                                                                                                           |
| -------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `title`        | `cairn`   | Site name: header, `<title>`. Translatable like every text key.                                                                                                                                                                      |
| `tagline`      | empty     | One sentence about the site, translatable. Opens the home page and feeds the meta description search engines and link previews show.                                                                                                 |
| `url`          | none      | Public base URL, **the domain alone**: `https://tools.example.org`, never `https://example.org/cairn`. Enables canonical and hreflang links, absolute sitemap URLs and the preview image. See [below](#the-domain-and-nothing-else). |
| `logo`         | none      | Image in the header, and the picture on a shared link. A URL or an [`/assets` path](../recipes/icons.md#your-own-files); [it has to be a raster](#the-logo-and-the-picture-on-a-shared-link).                                        |
| `favicon`      | cairn's   | The tab icon: a URL or an `/assets` path. See [the icon set](#the-icon-set) for what replacing it costs.                                                                                                                             |
| `index`        | `true`    | `index: false` asks search engines to stay away: `robots.txt` disallows everything, every page carries `noindex`, the sitemap turns off. For directories meant for ten people, not the whole web.                                    |
| `locales`      | `[en]`    | Languages served. First entry is the default and the fallback; see [Languages](i18n.md).                                                                                                                                             |
| `theme.accent` | `#247b7b` | Hex color for links, focus rings, buttons; see [Theming](theming.md).                                                                                                                                                                |
| `about`        | empty     | A welcome note under the hero, translatable; visitors can dismiss it (cookie, one year). Long text: paragraphs and the [markdown subset](text.md).                                                                                   |
| `links`        | `[]`      | Header navigation links, with optional icons; see below.                                                                                                                                                                             |
| `footer`       | `[]`      | Links at the bottom of every page.                                                                                                                                                                                                   |
| `pages`        | `[]`      | Pages cairn serves itself (legal notice, privacy…); see below. Bodies accept the [markdown subset](text.md).                                                                                                                         |
| `credit`       | `true`    | The small "powered by cairn" in the footer. `credit: false` removes it.                                                                                                                                                              |
| `show_version` | `false`   | Prints the running cairn version beside that credit: the release number for a tagged build, the commit for a build off `main`. Handy when you report a bug, or run several instances.                                                |
| `strings`      | built-ins | UI text overrides; see [Languages](i18n.md#ui-strings).                                                                                                                                                                              |
| `security`     | none      | `contact`, and optionally `policy` and `encryption`. Setting `contact` makes cairn serve [`/.well-known/security.txt`](#telling-researchers-where-to-write).                                                                         |
| `status`       | none      | Live status pills fed by your Gatus (`status.gatus`, `status.page`, `status.interval`, `status.linked`); see [Status page](../recipes/gatus.md).                                                                                     |

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

## The logo, and the picture on a shared link

`logo` puts an image in the header, before the title, sized by the stylesheet
to the height of the text. It is optional: without one the header carries the
title alone.

It has a second job that is easy to miss. When `url` is also set, a **raster**
logo becomes `og:image`, the picture Mastodon, Slack, Signal or a chat client
shows on the card when someone shares a link to your site.

| Logo                                | Header                 | Preview card |
| ----------------------------------- | ---------------------- | ------------ |
| none                                | title alone            | no image     |
| `logo.svg`                          | fine, scales perfectly | **no image** |
| `logo.png`, `.jpg`, `.webp`, `.gif` | fine                   | your image   |

The svg row is the one that surprises. A vector logo looks right everywhere on
the page, and most platforms simply ignore svg for a preview card, so your
links go out bare. `cairn -check` says so rather than leaving you to discover
it from a shared link:

```console
$ cairn -check -config ./config
warning: site.yaml logo "/assets/logo.svg" is not a raster image: links to the site preview with no image (og:image wants a png, jpg, webp or gif)
```

Nothing stops you keeping both: a vector `favicon` for the tab, where it scales
to every size a browser asks for, and a raster `logo` for the header and the
card. That is what the demo does.

On size: platforms crop a large card around 1200×630, so an image drawn for
that gets a full-width banner. A square logo gets a small square thumbnail
beside the text, which is perfectly respectable and what most self-hosted sites
want. cairn does not resize anything, it serves the file you point at, so pick
the shape you want to see.

Two things cairn deliberately will not do: fall back to its own mark when you
have no logo, because that would put cairn's stones on your link card, and
derive the tab icon from the logo, which is the separate `favicon` key.

## The domain and nothing else

`url` is the base of your domain, not the address of the page:

```yaml
url: https://tools.example.org # yes
url: https://example.org/cairn # no, even if that is where visitors land
```

If cairn is served under a sub-path, that prefix comes from `-base-path`, and
cairn adds it to every link itself. Writing it in `url` as well makes it land
twice: canonical links, hreflang alternates and every entry of the sitemap
would point at `example.org/cairn/cairn/`, which nothing serves. Search engines
act on those, so cairn treats it as a config error rather than emit them. It
does not exit: a fresh start then serves the getting-started page with the
reason in the log, and a reload keeps the previous pages serving.

```console
config: site.yaml: url "https://example.org/cairn" must be the domain alone, with no path
(serving under example.org/cairn is what -base-path is for, and cairn adds that prefix itself)
```

More on the sub-path in [Reverse proxies](../deployment/reverse-proxies.md#under-a-sub-path).

## Links a browser will not follow

`javascript:`, `vbscript:` and `data:` are a config error in every field of
this file that holds a link: `logo`, `favicon`, the `url` and `icon` of a
`links` or `footer` entry, and the three `security` fields.

```console
config: site.yaml: footer url "javascript:alert(1)" uses the javascript: scheme; cairn refuses javascript:, vbscript: and data: wherever a link goes (expected https://…, mailto:… or an absolute path)
```

The first two run code. `data:` is refused with them because `data:text/html`
carries a whole document, script and all, and because a list of two invites the
next reader to assume the third was weighed and kept. It costs nothing: an
inline `data:` favicon has never once reached a browser from cairn, for the
reason below.

The rest of the file was already answered: a service `url` has to be a URL or
a path, an unknown `links` icon is met with the list of glyphs. These fields
are the ones where nothing else would have spoken. A header link takes
`mailto:` besides `https://`, and `security.contact` takes any URI at all,
which is what [RFC 9116](https://www.rfc-editor.org/rfc/rfc9116) asks of it;
`logo` and `favicon` are checked by nothing at boot, since a missing file there
is a plausible typo and earns a `cairn -check` warning rather than a refusal.

What the refusal buys is an error instead of a silence. Nothing here would have
run: `html/template` emits only `http`, `https`, `mailto` and paths, and
replaces every other scheme with a dead placeholder, so the link renders in the
header of every page and goes nowhere, with nothing in the log to say why. The
favicon is the sharper case, since it reaches the web manifest as JSON, which
template escaping does not cover, and comes back from `/favicon.ico` as a
`Location` header.

That same rule is worth knowing for its own sake: a `tel:` link is blanked
exactly like these three. It is not refused, because it is nobody's attack and
refusing it would stop a site from loading over a link that merely does
nothing, but it will not work either. Write the number in the label and point
the link at a page.

The value is read the way a browser reads it, not the way it is written: a
browser drops tabs and newlines from a URL and strips leading control
characters before it looks at the scheme. `JavaScript:`, a leading space and a
tab inside the word are all the same URL as the plain one, so they all get the
same answer.

## The icon set

Out of the box cairn serves a complete set, all generated from one drawing so
none of it can drift:

| File                           | Where it shows up                                                                                                                                                 |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `favicon.svg`                  | The browser tab, at any size and on any tab colour.                                                                                                               |
| `favicon.ico`                  | 16, 32 and 48 pixels, for the feed readers and link previewers that fetch `/favicon.ico` and never read the html.                                                 |
| `touch-icon.png`               | 180 pixels: an iPhone or iPad home screen.                                                                                                                        |
| `icon-192.png`, `icon-512.png` | Android. Chromium offers "install this site" only when the manifest carries both, and cairn's are declared `maskable`, so Android's round crop takes nothing off. |

Nothing here is fetched from anywhere: like every other asset, the icons are
compiled into the binary and served from it.

### Using your own

Set `favicon` and it becomes the whole set. What cairn then tells browsers
about it depends on what it can actually establish, and it never guesses:

| What you set              | What the manifest says                                                                                  |
| ------------------------- | ------------------------------------------------------------------------------------------------------- |
| An **svg**, anywhere      | `sizes: any`. An svg is every size, so Android takes it for a home screen the way it takes cairn's own. |
| A **raster in `/assets`** | Its real width and height, measured when the config loads.                                              |
| A **raster behind a URL** | The file, with no size. Measuring it would mean an outbound request, and cairn makes none.              |

An entry without a size is still a valid manifest entry, and browsers still
use the icon. What it costs is the install prompt: Chromium offers "install
this site" only when it sees both a 192 and a 512.

iOS is the exception the table cannot cover, because an iPhone never reads the
manifest: it takes the home-screen icon from the `apple-touch-icon` link alone,
and cairn only points that at a `favicon` it can be sure is a png. So a png
favicon reaches an iPhone home screen and an svg one does not, which is the one
place a vector loses. The tab and Android take it happily; only iOS falls back
to cairn's mark, quietly, on somebody else's phone.

Naming your icons settles both platforms at once, and it is also how you get
the install prompt with your own artwork:

```yaml
icons:
  - { src: /assets/brand-192.png, sizes: 192x192 }
  - { src: /assets/brand-512.png, sizes: 512x512, purpose: any maskable }
```

`icons` replaces everything derived from `favicon`, which keeps serving the
browser tab. iOS is served out of the same list: cairn points its
`apple-touch-icon` at the largest entry, so an iPhone home screen shows your
artwork and not cairn's, whatever the `favicon` happens to be.

`sizes` is `WIDTHxHEIGHT`, or `any` for an svg. `purpose` is
optional and accepts `any`, `maskable` and `monochrome`, combined if you like.
A **maskable** icon must keep its content inside the middle 80% of the square,
because Android crops away the rest.

cairn does not resize images: doing it would mean shipping a scaler for one
rarely used path, and a badly resampled logo is worse than the one you drew.
Anything wrong in this list is a config error that names the entry, so a typo
stops at startup rather than reaching a phone. A list that is merely short of
the 192 and 512 pair is not wrong, only quieter than you meant, so that one is
a `cairn -check` warning instead.

So is a `sizes` the file disagrees with. cairn cannot measure for you and
publishes your number in the manifest as fact, and a wrong one is invisible
from every side: the manifest validates, the file loads, and a phone simply
picks it for a slot it does not fill, which is how a home-screen icon comes out
blurred. With the assets directory in reach, `-check` opens each file and
compares:

```console
$ cairn -check -config ./config -assets ./assets
warning: site.yaml icons entry 2 declares sizes "512x512" but /assets/brand-512.png measures 256x256: the manifest states the declared size as fact, so a phone picks this file for a slot it does not fill (correct sizes, or supply a file that size)
```

An entry whose `src` names no file at all gets its own line. Both checks need
that directory, so they say nothing when it is not mounted; see
[Icons: your own files](../recipes/icons.md#your-own-files).

## Telling researchers where to write

Somebody who finds a flaw in one of your services has to guess where to send
it. [RFC 9116](https://www.rfc-editor.org/rfc/rfc9116) settles that with a file
at a fixed address, and one key makes cairn serve it:

```yaml
security:
  contact: mailto:security@example.org
  policy: https://example.org/disclosure # optional
  encryption: https://example.org/pgp.asc # optional
```

`contact` is the only field you write. It takes any URI, so `mailto:`,
`https://` to a reporting form, or `tel:` all work; writing a bare address
without its scheme is a config error rather than a file nobody can parse.

The rest cairn fills, because it already knows it:

```
Contact: mailto:security@example.org
Expires: 2027-07-30T17:45:16Z
Preferred-Languages: fr, en
Canonical: https://tools.example.org/.well-known/security.txt
```

`Expires` is the field this exists to get right. The RFC makes it mandatory,
and a security.txt written by hand carries a date somebody chose once and never
looked at again: past it, the file is worse than absent, because it says the
address is stale. cairn recomputes it on every request, a year out, so it
cannot rot. `Preferred-Languages` comes from your `locales` and `Canonical`
from `url`, or from the request when you have not set one.

Without `security.contact` the path answers `404`, which is the honest answer:
an empty security.txt is a promise cairn cannot make on your behalf.

One caveat if you serve cairn [under a sub-path](../deployment/reverse-proxies.md#under-a-sub-path):
the RFC wants this file at the root of the domain, and cairn can only serve it
under its own prefix. Point your proxy at it, or leave the file to whatever
owns the root.

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
