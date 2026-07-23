# Reference

Everything on one page. The per-topic pages explain; this one enumerates.

## Files in `/config`

| File              | Role                                        |
| ----------------- | ------------------------------------------- |
| `site.yaml`       | site chrome; optional                       |
| `categories.yaml` | category names and order; optional          |
| `custom.css`      | loaded after cairn's stylesheet; optional   |
| any other `*.yaml`/`*.yml` | a list of services; at least one required |

Changes to any of them apply within ~2 seconds. A bad file at boot stops the
server with an error naming the file, line, problem and expected shape; a bad
file at reload keeps the previous config serving and logs the same error.

## `services.yaml` (any name)

| Key        | Required | Default       | Type                    |
| ---------- | -------- | ------------- | ----------------------- |
| `id`       | yes      | n/a           | slug `[a-z0-9-]`, unique across files |
| `url`      | yes      | n/a           | `http(s)://…` or absolute path |
| `name`     | yes      | n/a           | text (plain or per-locale map) |
| `desc`     | no       | empty         | text                    |
| `details`  | no       | empty         | text; blank lines split paragraphs; enables the card's "Learn more" link |
| `category` | no       | `other`       | slug                    |
| `icon`     | no       | neutral glyph | slug, URL, or `/assets/…` path |
| `tags`     | no       | `[]`          | list of search words    |

## `site.yaml`

| Key            | Default   | Type                       |
| -------------- | --------- | -------------------------- |
| `title`        | `cairn`   | plain string               |
| `tagline`      | empty     | text                       |
| `logo`         | none      | URL or `/assets/…` path    |
| `locales`      | `[en]`    | list; first = default      |
| `theme.accent` | `#247b7b` | hex color                  |
| `footer`       | `[]`      | list of `{label: text, url: string}` |
| `strings`      | built-ins | map of key → text ([keys](configuration/i18n.md#ui-strings)) |
| `status.gatus` | none      | Gatus base URL cairn polls for the [status pills](recipes/gatus.md) |
| `status.page`  | `status.gatus` | public Gatus URL the pill links to (set when the poll URL is internal) |
| `status.interval` | `60s`  | poll cadence, duration ≥ `5s`         |

Unknown keys in `site.yaml` are errors.

## `categories.yaml`

A list; each entry:

| Key     | Required | Default            | Type |
| ------- | -------- | ------------------ | ---- |
| `id`    | yes      | n/a                | slug matching services' `category` |
| `name`  | no       | id, capitalized    | text |
| `order` | no       | none               | integer |

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

| Path            | Behavior                                                  |
| --------------- | --------------------------------------------------------- |
| `/`             | 302 to the negotiated locale (cookie → `Accept-Language` → default) |
| `/{locale}/`    | home; server-rendered, `ETag`, `Cache-Control: no-cache`  |
| `/{locale}/{id}/` | service detail page; same caching                       |
| `/static/…`     | embedded assets, cached one day                           |
| `/assets/…`     | your mounted files, if the mount exists                   |
| `/custom.css`   | your stylesheet, if present                               |
| `/healthz`      | `200 ok`                                                  |
| `/sitemap.xml`  | every page, absolute URLs from `Host`/`X-Forwarded-Proto` |
| `/robots.txt`   | allow all + sitemap URL                                   |

## Binary flags

| Flag           | Default   | Role                                       |
| -------------- | --------- | ------------------------------------------ |
| `-addr`        | `:8080`   | listen address                             |
| `-config`      | `/config` | config directory                           |
| `-assets`      | `/assets` | directory served at `/assets/`, if it exists |
| `-healthcheck` | off       | probe `127.0.0.1:{port}/healthz`, exit 0/1, for `FROM scratch` healthchecks |
| `-emit-gatus`  | off       | print a [Gatus endpoints config](recipes/gatus.md) derived from the services, then exit |

No environment variables, no other state. The binary serves HTTP on one port,
reads one directory, and optionally polls the one Gatus URL you configured.
