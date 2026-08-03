# Reference

Everything on one page. The per-topic pages explain; this one enumerates.

## Files in `/config`

| File                       | Role                                                                          |
| -------------------------- | ----------------------------------------------------------------------------- |
| `site.yaml`                | site chrome; optional                                                         |
| `categories.yaml`          | category names and order; optional                                            |
| `custom.css`               | loaded after cairn's stylesheet; optional                                     |
| `media/`                   | service preview images, served at `/media/`; optional                         |
| `fonts/`                   | self-hosted font files `theme.font.file` names, served at `/fonts/`; optional |
| any other `*.yaml`/`*.yml` | a list of services; at least one required                                     |

Changes to any of them apply within ~2 seconds. A bad or missing config at
boot serves a built-in getting-started page while the log names the file,
line, problem and expected shape; the real site takes over the moment the
config is valid. A bad file at reload keeps the previous config serving and
logs the same error.

## `services.yaml` (any name)

| Key          | Required | Default       | Type                                                                                                                                                                                                                                             |
| ------------ | -------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `id`         | yes      | n/a           | slug `[a-z0-9][a-z0-9-]*`, unique across files: it becomes a URL, so no leading dash                                                                                                                                                             |
| `url`        | yes      | n/a           | `http(s)://…` or absolute path                                                                                                                                                                                                                   |
| `name`       | yes      | n/a           | text (plain or per-locale map)                                                                                                                                                                                                                   |
| `desc`       | no       | empty         | text                                                                                                                                                                                                                                             |
| `details`    | no       | empty         | long text ([CommonMark, minus raw HTML](configuration/text.md)); enables the card's detail-page glyph                                                                                                                                            |
| `images`     | no       | `[]`          | detail-page previews; list of `name.png` or `/media/name.png` (both name the same file in `/config/media/`, and both have to be there) or `{src, caption: text}`; URLs and other absolute paths pass through; also enables the detail-page glyph |
| `category`   | no       | `other`       | slug                                                                                                                                                                                                                                             |
| `icon`       | no       | neutral glyph | slug, URL, `/assets/…` path, or `{light: …, dark: …}` ([two themes](configuration/theming.md#artwork-that-cannot-serve-both-themes))                                                                                                             |
| `tags`       | no       | `[]`          | list of search words                                                                                                                                                                                                                             |
| `selfhosted` | no       | none          | `true`/`false`: shows a self-hosted / hosted-elsewhere flag on the card ([details](configuration/services.md#hosting-flag))                                                                                                                      |

## `site.yaml`

| Key                     | Default     | Type                                                                                                                                                                                                                      |
| ----------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `title`                 | `cairn`     | text (plain or per-locale map)                                                                                                                                                                                            |
| `tagline`               | empty       | text; opens the home page, feeds the meta description                                                                                                                                                                     |
| `url`                   | none        | public base URL; adds canonical + hreflang, absolute sitemap                                                                                                                                                              |
| `logo`                  | none        | URL, `/assets/…` path, or `{light: …, dark: …}` ([two themes](configuration/theming.md#artwork-that-cannot-serve-both-themes)); raster logos double as the og:image when `url` is set, always the light one               |
| `favicon`               | cairn's     | tab icon; URL, `/assets/…` path, or `{light: …, dark: …}`. A png also becomes the home-screen icon; see [the icon set](configuration/site.md#the-icon-set)                                                                |
| `icons`                 | derived     | [home-screen set](configuration/site.md#using-your-own), list of `{src: URL/path, sizes: WxH or any, purpose: any/maskable/monochrome}`; overrides what `favicon` implies                                                 |
| `index`                 | `true`      | `false` = robots.txt disallow, noindex meta, no sitemap                                                                                                                                                                   |
| `locales`               | `[en]`      | list; first = default                                                                                                                                                                                                     |
| `theme.accent`          | `#247b7b`   | hex color                                                                                                                                                                                                                 |
| `theme.font.family`     | none        | CSS font stack for body text, e.g. `"Inter, system-ui, sans-serif"` ([Theming](configuration/theming.md#2-typography))                                                                                                    |
| `theme.font.file`       | none        | a `woff2`/`woff`/`ttf`/`otf` in the config directory's `fonts/` folder (e.g. `fonts/custom-font.woff2`), served by cairn at `/fonts/`; needs `theme.font.family` to name it                                               |
| `about`                 | empty       | dismissable welcome note under the hero, long text ([CommonMark, minus raw HTML](configuration/text.md))                                                                                                                  |
| `links`                 | `[]`        | header nav links, list of `{label: text, url: string, icon: [glyph](configuration/site.md#header-links)/URL/path}`                                                                                                        |
| `footer`                | `[]`        | list of `{label: text, url: string}`; both required, as in `links`                                                                                                                                                        |
| `pages`                 | `[]`        | [hosted pages](configuration/site.md#hosted-pages): `{id: slug, title: text, body: long text, sections: [{title, body}]}`; body and/or sections required; bodies take [CommonMark, minus raw HTML](configuration/text.md) |
| `credit`                | `true`      | bool; `false` removes the footer "powered by cairn"                                                                                                                                                                       |
| `show_version`          | `false`     | bool; `true` shows the running version next to the credit, linked to its release or commit                                                                                                                                |
| `strings`               | built-ins   | map of key → text ([keys](configuration/i18n.md#ui-strings))                                                                                                                                                              |
| `security.contact`      | none        | URI (`mailto:`, `https://`, `tel:`); setting it serves [`/.well-known/security.txt`](configuration/site.md#telling-researchers-where-to-write)                                                                            |
| `security.policy`       | none        | URL of your disclosure policy                                                                                                                                                                                             |
| `security.encryption`   | none        | URL of the key a reporter should encrypt to                                                                                                                                                                               |
| `hosting_flag.self`     | none        | where the Self-hosted flag leads: a page id, an absolute path or a URL ([details](configuration/services.md#making-the-flag-lead-somewhere))                                                                              |
| `hosting_flag.external` | none        | the same for the External flag                                                                                                                                                                                            |
| `status.gatus`          | none        | Gatus base URL cairn polls for the [status pills](recipes/status.md)                                                                                                                                                      |
| `status.provider`       | `gatus`     | which monitor answers at `status.url`: `gatus`, `kuma` or `json`; `status.gatus` names its own, so a Gatus site needs neither key                                                                                         |
| `status.url`            | none        | address of the monitor named by `status.provider`: the generic spelling of `status.gatus`, and you set one or the other, never both                                                                                       |
| `status.slug`           | none        | Uptime Kuma only: the published [status page](recipes/status.md#uptime-kuma) cairn reads, named by the last part of its URL                                                                                               |
| `status.map`            | none        | `status.provider: json` only: `list`, `key`, `state`, then `up`, `degraded`, `maintenance`, `unknown`, the values that mean each ([mapping](recipes/status.md#any-other-status-api))                                      |
| `status.token_file`     | none        | absolute path of a file holding the bearer token for the status API, never the token itself; refused under `/assets`, which is served                                                                                     |
| `status.token_scheme`   | `Bearer`    | authorization scheme sent with that token (`OAuth` for Atlassian Statuspage)                                                                                                                                              |
| `status.page`           | the address | public URL the pill links to (set when the poll URL is internal)                                                                                                                                                          |
| `status.interval`       | `60s`       | poll cadence, duration ≥ `5s`; also how often an open page refreshes its pills                                                                                                                                            |
| `status.linked`         | `true`      | `false` makes the pills display-only: the state stays, the link to the status page goes                                                                                                                                   |
| `status.insecure`       | `false`     | `true` stops cairn verifying the certificate Gatus presents; logged at startup and warned about by `-check` ([why, and what to do instead](deployment/airgap.md#6-self-signed-certificates))                              |
| `status.ca`             | none        | PEM bundle cairn verifies Gatus against, on top of the system roots: an `http(s)://` URL it fetches, or an `/assets` path it reads ([the two shapes](deployment/airgap.md#6-self-signed-certificates))                    |

Unknown keys are errors everywhere: in `site.yaml`, and in every service,
category, page, section, link, image and icon entry alike. The message names
the kind of entry that refused the key and lists what that entry accepts, so
a misspelt `show_versions` is answered with `show_version` rather than with
the keys of some other part of the file.

Every field above that becomes a link takes `https://`, `http://`, an absolute
path, and for `links` and `footer` entries `mailto:`. Every other scheme is
refused, `tel:` included: `html/template` writes only those in an `href`, and
replaces anything else with a placeholder that goes nowhere, so the link would
render dead with nothing in the log to say why. `security.*` is the exception
and keeps `tel:`, because that file is plain text rather than markup. See
[Links a browser will not follow](configuration/site.md#links-a-browser-will-not-follow).

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

| Path                        | Behavior                                                                                                  |
| --------------------------- | --------------------------------------------------------------------------------------------------------- |
| `/`                         | 302 to the negotiated locale (cookie → `Accept-Language` → default)                                       |
| `/{locale}/`                | home; server-rendered, `ETag` (suffixed `-gzip` when the answer is compressed), `Cache-Control: no-cache` |
| `/{locale}/{id}/`           | service detail page or [hosted page](configuration/site.md#hosted-pages); same caching                    |
| `/{locale}/…?choose`        | sets the one-year locale cookie, then redirects clean                                                     |
| `/static/…`                 | embedded assets; a name carrying a content digest is cached for a year, a plain one for a day (see below) |
| `/assets/…`                 | your mounted files, if the mount exists                                                                   |
| `/media/…`                  | images from `<config>/media/`, the ones services and long text name                                       |
| `/fonts/…`                  | the self-hosted font `theme.font.file` names, from `<config>/fonts/`                                      |
| `/custom.css`               | your stylesheet, if present                                                                               |
| `/manifest.webmanifest`     | the web app manifest: site name, theme color, the [icon set](configuration/site.md#the-icon-set)          |
| `/favicon.ico`              | cairn's own, or a redirect to your `favicon`, for the tools that skip the html                            |
| `/healthz`                  | `200 ok` while the process serves, whatever the config says: the liveness signal                          |
| `/readyz`                   | `200 ready`, or `503` while no valid config has ever loaded and the getting-started page stands in        |
| `/sitemap.xml`              | every page, absolute URLs from `Host`/`X-Forwarded-Proto`                                                 |
| `/robots.txt`               | allow all + sitemap URL                                                                                   |
| `/.well-known/security.txt` | RFC 9116, once `security.contact` is set; `404` otherwise                                                 |

Text responses are gzipped for clients that ask, which is most of them: every
text response a first visit to the demo pulls, the page and its stylesheet, the
five small scripts, the manifest and the svg icons the demo serves itself, goes
from 72 KB on the wire to 22 KB. Images, fonts and the `ico` are left alone,
since compressing them again only spends CPU to add bytes, so the 62 KB font
weighs 62 KB either way. Every response carries `Vary: Accept-Encoding` so a
shared cache cannot hand the compressed answer to a client that did not ask for
it.

### Why an asset url carries a digest

cairn's own assets go out under a name that carries a digest of what is inside
them: `style.css` is linked as `style.0629016b.css`. It exists to fix one thing
an operator meets on every upgrade. Pages are served `no-cache`, so they update
the moment cairn restarts; the stylesheet beside them was cached for a day
under a name that never changed, so the new page arrived wearing yesterday's
CSS and only a hard refresh fixed it.

Since the name changes exactly when the bytes do, the fresh page points at a
name the browser has never seen, and a file nobody touched keeps its name and
stays in the cache. That is what makes `max-age=31536000, immutable` safe here:
nothing has to expire, because nothing is ever replaced under the same name.

The digest is in the name rather than in a `?v=` query, because a query leaves
an intermediary cache free to key on the path alone and pin a stale file for
the whole year. Plain `/static/style.css` still answers, on the day-long cache,
so anything that hardcoded it goes on working. A digest cairn did not issue
gets a 404 rather than a file.

The display font is the one exception: `style.css` reaches it with a relative
`url()` of its own, which nothing out here can rewrite, so it keeps its plain
name and the shorter cache. Everything else cairn ships is stamped, including
the apple-touch-icon and the manifest's icons, which are built in Go rather
than written in a template. Your own files, under `/assets/` and `/media/`,
are never stamped: cairn has not read their bytes and has no digest to offer.

## Binary flags

| Flag            | Default   | Role                                                                                                                                                                                                                                                                                                                                  |
| --------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-addr`         | `:8080`   | listen address                                                                                                                                                                                                                                                                                                                        |
| `-config`       | `/config` | config directory                                                                                                                                                                                                                                                                                                                      |
| `-assets`       | `/assets` | directory served at `/assets/`, if it exists                                                                                                                                                                                                                                                                                          |
| `-base-path`    | none      | serve under a [sub-path](deployment/reverse-proxies.md#under-a-sub-path) of the domain, e.g. `/cairn`                                                                                                                                                                                                                                 |
| `-healthcheck`  | off       | probe `127.0.0.1:{port}/healthz`, exit 0/1, for `FROM scratch` healthchecks                                                                                                                                                                                                                                                           |
| `-init`         | off       | print a commented starter `services.yaml`, then exit                                                                                                                                                                                                                                                                                  |
| `-emit-gatus`   | off       | print a [Gatus endpoints config](recipes/status.md) derived from the services, then exit                                                                                                                                                                                                                                              |
| `-hide-targets` | off       | with `-emit-gatus`: add the `ui` block that keeps each endpoint's [address off the Gatus dashboard](recipes/status.md#hide-what-gatus-probes)                                                                                                                                                                                         |
| `-emit-icons`   | off       | print a shell script that downloads your icon slugs for [self-hosting](recipes/icons.md#going-fully-self-hosted), then exit                                                                                                                                                                                                           |
| `-check`        | off       | validate the config directory, print warnings (partial translations, `strings` or `locales` covering part of the site, orphan or heavy media, references that resolve nowhere or name no file, icon sizes the file contradicts, ids that collide, keys that do nothing, a custom font that is not there, CDN icons), then exit 0 or 1 |
| `-version`      | off       | print the version, then exit                                                                                                                                                                                                                                                                                                          |

With `-base-path`, every path above moves under the prefix (`/cairn/en/`,
`/cairn/static/…`) and cairn strips it back off itself, so the proxy in front
needs no rewriting. `/healthz` is the one exception: it answers at the root
too, alongside `/readyz`, because an orchestrator reaches cairn directly.

The `-check` warnings that open a file, a reference under `/assets/` and an
icon's declared size, are skipped entirely when the `-assets` directory does
not exist. That directory is there in the container and on nobody's laptop, so
checking anyway would print a page of wrong warnings and teach you to skim the
output. Mount it, as [Air-gapped](deployment/airgap.md#2-the-check-that-says-whether-you-are-actually-ready)
does, and they come back.

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
