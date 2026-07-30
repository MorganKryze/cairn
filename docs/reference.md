# Reference

Everything on one page. The per-topic pages explain; this one enumerates.

## Files in `/config`

| File                       | Role                                                  |
| -------------------------- | ----------------------------------------------------- |
| `site.yaml`                | site chrome; optional                                 |
| `categories.yaml`          | category names and order; optional                    |
| `custom.css`               | loaded after cairn's stylesheet; optional             |
| `media/`                   | service preview images, served at `/media/`; optional |
| any other `*.yaml`/`*.yml` | a list of services; at least one required             |

Changes to any of them apply within ~2 seconds. A bad or missing config at
boot serves a built-in getting-started page while the log names the file,
line, problem and expected shape; the real site takes over the moment the
config is valid. A bad file at reload keeps the previous config serving and
logs the same error.

## `services.yaml` (any name)

| Key          | Required | Default       | Type                                                                                                                                                             |
| ------------ | -------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`         | yes      | n/a           | slug `[a-z0-9-]`, unique across files                                                                                                                            |
| `url`        | yes      | n/a           | `http(s)://…` or absolute path                                                                                                                                   |
| `name`       | yes      | n/a           | text (plain or per-locale map)                                                                                                                                   |
| `desc`       | no       | empty         | text                                                                                                                                                             |
| `details`    | no       | empty         | long text ([markdown subset](configuration/text.md)); enables the card's "Learn more" link                                                                       |
| `images`     | no       | `[]`          | detail-page previews; list of `name.png` (a file in `/config/media/`) or `{src, caption: text}`; URLs and absolute paths pass through; also enables "Learn more" |
| `category`   | no       | `other`       | slug                                                                                                                                                             |
| `icon`       | no       | neutral glyph | slug, URL, or `/assets/…` path                                                                                                                                   |
| `tags`       | no       | `[]`          | list of search words                                                                                                                                             |
| `selfhosted` | no       | none          | `true`/`false`: shows a self-hosted / hosted-elsewhere flag on the card ([details](configuration/services.md#hosting-flag))                                      |

## `site.yaml`

| Key                   | Default        | Type                                                                                                                                                                                                               |
| --------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `title`               | `cairn`        | text (plain or per-locale map)                                                                                                                                                                                     |
| `tagline`             | empty          | text; opens the home page, feeds the meta description                                                                                                                                                              |
| `url`                 | none           | public base URL; adds canonical + hreflang, absolute sitemap                                                                                                                                                       |
| `logo`                | none           | URL or `/assets/…` path; raster logos double as the og:image when `url` is set                                                                                                                                     |
| `favicon`             | cairn's        | tab icon; URL or `/assets/…` path. Also becomes the home-screen icon; see [the icon set](configuration/site.md#the-icon-set)                                                                                       |
| `icons`               | derived        | [home-screen set](configuration/site.md#using-your-own), list of `{src: URL/path, sizes: WxH or any, purpose: any/maskable/monochrome}`; overrides what `favicon` implies                                          |
| `index`               | `true`         | `false` = robots.txt disallow, noindex meta, no sitemap                                                                                                                                                            |
| `locales`             | `[en]`         | list; first = default                                                                                                                                                                                              |
| `theme.accent`        | `#247b7b`      | hex color                                                                                                                                                                                                          |
| `about`               | empty          | dismissable welcome note under the hero, long text ([markdown subset](configuration/text.md))                                                                                                                      |
| `links`               | `[]`           | header nav links, list of `{label: text, url: string, icon: [glyph](configuration/site.md#header-links)/URL/path}`                                                                                                 |
| `footer`              | `[]`           | list of `{label: text, url: string}`                                                                                                                                                                               |
| `pages`               | `[]`           | [hosted pages](configuration/site.md#hosted-pages): `{id: slug, title: text, body: long text, sections: [{title, body}]}`; body and/or sections required; bodies take the [markdown subset](configuration/text.md) |
| `credit`              | `true`         | bool; `false` removes the footer "powered by cairn"                                                                                                                                                                |
| `show_version`        | `false`        | bool; `true` shows the running version next to the credit, linked to its release or commit                                                                                                                         |
| `strings`             | built-ins      | map of key → text ([keys](configuration/i18n.md#ui-strings))                                                                                                                                                       |
| `security.contact`    | none           | URI (`mailto:`, `https://`, `tel:`); setting it serves [`/.well-known/security.txt`](configuration/site.md#telling-researchers-where-to-write)                                                                     |
| `security.policy`     | none           | URL of your disclosure policy                                                                                                                                                                                      |
| `security.encryption` | none           | URL of the key a reporter should encrypt to                                                                                                                                                                        |
| `status.gatus`        | none           | Gatus base URL cairn polls for the [status pills](recipes/gatus.md)                                                                                                                                                |
| `status.page`         | `status.gatus` | public Gatus URL the pill links to (set when the poll URL is internal)                                                                                                                                             |
| `status.interval`     | `60s`          | poll cadence, duration ≥ `5s`                                                                                                                                                                                      |
| `status.linked`       | `true`         | `false` makes the pills display-only: the state stays, the link to the status page goes                                                                                                                            |

Unknown keys are errors everywhere: in `site.yaml`, and in every service,
category, page, section, link, image and icon entry alike. The message names
the kind of entry that refused the key and lists what that entry accepts, so
a misspelt `show_versions` is answered with `show_version` rather than with
the keys of some other part of the file.

## `categories.yaml`

A list; each entry:

| Key     | Required | Default                               | Type                               |
| ------- | -------- | ------------------------------------- | ---------------------------------- |
| `id`    | yes      | n/a                                   | slug matching services' `category` |
| `name`  | no       | id, capitalized, dashes become spaces | text                               |
| `order` | no       | none                                  | integer                            |

```yaml
- id: documents
  name: { fr: Documents, en: Documents }
  order: 1
```

Sorting: entries with `order` first (ascending), then the rest
alphabetically, `other` always last. Entries without services are not
rendered. Without `categories.yaml`, groups derive from service `category`
ids, alphabetically.

## Endpoints

| Path                        | Behavior                                                                                           |
| --------------------------- | -------------------------------------------------------------------------------------------------- |
| `/`                         | 302 to the negotiated locale (cookie → `Accept-Language` → default)                                |
| `/{locale}/`                | home; server-rendered, `ETag`, `Cache-Control: no-cache`                                           |
| `/{locale}/{id}/`           | service detail page or [hosted page](configuration/site.md#hosted-pages); same caching             |
| `/{locale}/…?choose`        | sets the one-year locale cookie, then redirects clean                                              |
| `/static/…`                 | embedded assets, cached one day                                                                    |
| `/assets/…`                 | your mounted files, if the mount exists                                                            |
| `/custom.css`               | your stylesheet, if present                                                                        |
| `/healthz`                  | `200 ok` while the process serves, whatever the config says: the liveness signal                   |
| `/readyz`                   | `200 ready`, or `503` while no valid config has ever loaded and the getting-started page stands in |
| `/sitemap.xml`              | every page, absolute URLs from `Host`/`X-Forwarded-Proto`                                          |
| `/robots.txt`               | allow all + sitemap URL                                                                            |
| `/.well-known/security.txt` | RFC 9116, once `security.contact` is set; `404` otherwise                                          |

Text responses are gzipped for clients that ask, which is most of them: a first
visit to the demo goes from 53 KB on the wire to 14 KB. Images, fonts and the
`ico` are left alone, since compressing them again only spends CPU to add
bytes. Every response carries `Vary: Accept-Encoding` so a shared cache cannot
hand the compressed answer to a client that did not ask for it.

## Binary flags

| Flag           | Default   | Role                                                                                                                        |
| -------------- | --------- | --------------------------------------------------------------------------------------------------------------------------- |
| `-addr`        | `:8080`   | listen address                                                                                                              |
| `-config`      | `/config` | config directory                                                                                                            |
| `-assets`      | `/assets` | directory served at `/assets/`, if it exists                                                                                |
| `-base-path`   | none      | serve under a [sub-path](deployment/reverse-proxies.md#under-a-sub-path) of the domain, e.g. `/cairn`                       |
| `-healthcheck` | off       | probe `127.0.0.1:{port}/healthz`, exit 0/1, for `FROM scratch` healthchecks                                                 |
| `-init`        | off       | print a commented starter `services.yaml`, then exit                                                                        |
| `-emit-gatus`  | off       | print a [Gatus endpoints config](recipes/gatus.md) derived from the services, then exit                                     |
| `-emit-icons`  | off       | print a shell script that downloads your icon slugs for [self-hosting](recipes/icons.md#going-fully-self-hosted), then exit |
| `-check`       | off       | validate the config directory, print warnings (missing translations, orphan or heavy media), then exit 0 or 1               |
| `-version`     | off       | print the version, then exit                                                                                                |

With `-base-path`, every path above moves under the prefix (`/cairn/en/`,
`/cairn/static/…`) and cairn strips it back off itself, so the proxy in front
needs no rewriting. `/healthz` is the one exception: it answers at the root
too, alongside `/readyz`, because an orchestrator reaches cairn directly.

## The display font

`src/internal/render/assets/fonts/fraunces.woff2` is not the upstream file: it is
[Fraunces](https://github.com/undercasetype/Fraunces) cut down to what the
pages use, which halves it (118 KB to 62 KB) on a page that would otherwise be
two-thirds font. The Latin ranges are kept, the `SOFT` and `WONK` axes are
pinned to the single values the stylesheet asks for, and `opsz` and `wght` stay
variable so headings keep their optical sizing.

To rebuild it after a font update:

```sh
pip install fonttools brotli
python - <<'EOF'
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer
from fontTools import subset

f = TTFont("Fraunces[SOFT,WONK,opsz,wght].ttf")
instancer.instantiateVariableFont(f, {"WONK": 0, "SOFT": 50}, inplace=True)
o = subset.Options()
o.layout_features = ["kern", "liga", "calt", "ccmp", "locl"]
o.name_IDs = ["*"]
s = subset.Subsetter(options=o)
s.populate(unicodes=subset.parse_unicodes(
    "U+0000-00FF,U+0100-017F,U+0180-024F,U+2000-206F,U+20A0-20BF,U+2122"))
s.subset(f)
f.flavor = "woff2"
f.save("src/internal/render/assets/fonts/fraunces.woff2")
EOF
```

Those ranges cover the seven built-in languages and the rest of Latin-script
Europe. A site whose headings need Greek, Cyrillic or CJK should point
`--font-display` at its own face through
[custom.css](configuration/theming.md).

No environment variables, no other state. The binary serves HTTP on one port,
reads one directory, and optionally polls the one Gatus URL you configured.
