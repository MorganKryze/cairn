# cairn

> A cairn is a small stack of stones left by hikers who walked the trail
> before you, so you find your way without digging.

cairn is a directory page for self-hosted services, written for the people you
host them for — family, clients, friends — not for the admin who installed
them. It says what this place is and what each tool does, in plain words, in
the visitor's language.

- **Guest-first.** No jargon, tools grouped by need, one-line descriptions.
- **Multilingual by design.** One YAML file with translated fields inline. The
  server picks the visitor's language from the browser; a switcher in the
  header remembers their choice.
- **Calm and accessible.** Real typography, light and dark, keyboard friendly,
  works without JavaScript.
- **Boring to operate.** One image around 12 MB, YAML mounted read-only, no
  database, no tracking, no runtime dependency. Config edits apply within
  seconds, without a restart.

## Quickstart

Create a folder with one file:

```yaml
# config/services.yaml
- id: pdf
  url: https://pdf.example.org
  category: documents
  icon: stirling-pdf
  name: { fr: Boîte à outils PDF, en: PDF toolbox }
  desc: { fr: "Fusionner, découper, compresser vos PDF.", en: "Merge, split, compress your PDFs." }
  tags: [pdf, convert]
```

Then run it:

```yaml
# compose.yaml
services:
  cairn:
    build:
      context: https://github.com/MorganKryze/cairn.git
      dockerfile: docker/Dockerfile
    ports: ["8080:8080"]
    volumes: ["./config:/config:ro"]
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    healthcheck:
      test: ["CMD", "/cairn", "-healthcheck"]
      interval: 30s
```

```sh
docker compose up -d
```

Open <http://localhost:8080>. That is a finished page; everything below is
optional.

From a clone, `docker compose -f docker/compose.yaml up --build` builds the
image (running the tests on the way) and serves the example config; without
Docker, `go run ./src -config example` does the same.

## Configuration

cairn reads every `*.yaml` file in `/config`. `site.yaml` describes the site;
every other file is a list of services, so one file per category works out of
the box (files merge in name order). A config error names the file, the line,
the problem and the expected shape — at boot it stops the server, at reload
the previous config keeps serving.

Any translatable value accepts either a plain string or a per-locale map:

```yaml
name: PDF toolbox                       # same text for every locale
name: { fr: Boîte à outils PDF, en: PDF toolbox }
```

### Services (`services.yaml` or any other file name)

| Key        | Required | Default        | Example                          |
| ---------- | -------- | -------------- | -------------------------------- |
| `id`       | yes      | —              | `pdf` (must be unique)           |
| `url`      | yes      | —              | `https://pdf.example.org`        |
| `name`     | yes      | —              | `{ fr: Boîte à outils PDF, en: PDF toolbox }` |
| `desc`     | no       | empty          | `Merge, split, compress your PDFs.` |
| `details`  | no       | empty          | longer text; gives the card a “Learn more” detail page |
| `category` | no       | `other`        | `documents`                      |
| `icon`     | no       | neutral glyph  | `stirling-pdf`                   |
| `tags`     | no       | `[]`           | `[pdf, convert]` (searchable, invisible) |

### Categories (`categories.yaml`, optional)

Without it, groups derive from the `category` ids, sorted alphabetically.
With it, they get translated names and an explicit order:

```yaml
- id: documents
  name: { fr: Documents, en: Documents }
  order: 1
```

### Site (`site.yaml`, optional)

| Key            | Default     | Example                                  |
| -------------- | ----------- | ---------------------------------------- |
| `title`        | `cairn`     | `Libre Internet`                         |
| `tagline`      | empty       | `{ fr: "Des outils libres…", en: "Free tools…" }` |
| `logo`         | none        | `/assets/logo.png` or a URL              |
| `locales`      | `[en]`      | `[fr, en]` — first entry is the default and fallback |
| `theme.accent` | `#247b7b`   | any hex color                            |
| `footer`       | `[]`        | `- label: { fr: Statut, en: Status }`<br>&nbsp;&nbsp;`url: https://status.example.org` |
| `strings`      | built-ins   | see below                                |

### UI strings

Every piece of interface text ships with French and English defaults and can
be overridden per locale from `site.yaml`:

```yaml
strings:
  search.placeholder: { fr: "Chercher…", en: "Search…" }
```

Keys: `nav.skip`, `nav.languages`, `search.label`, `search.placeholder`,
`search.empty`, `cat.other`.

### Icons

`icon:` accepts three forms:

- a bare slug, resolved against
  [dashboard-icons](https://github.com/homarr-labs/dashboard-icons) — the same
  convention Homepage and Homarr use: `icon: stirling-pdf`;
- a full URL: `icon: https://example.org/icon.svg`;
- a path under `/assets`, an optional read-only mount of your own files:
  `icon: /assets/icons/tool.svg`.

Services without an icon get a neutral glyph.

### Theming

`theme.accent` recolors the page; a `custom.css` dropped next to your YAML
files is served last and wins — see
[docs/configuration/theming.md](docs/configuration/theming.md).

## Pages and endpoints

| Path           | What                                                        |
| -------------- | ----------------------------------------------------------- |
| `/`            | redirects to the visitor's language (cookie, then `Accept-Language`, then default) |
| `/{locale}/`   | the directory, server-rendered, cached with an ETag         |
| `/{locale}/{id}/` | per-service detail page — “when would I use this?”       |
| `/healthz`     | returns `ok` — pair with `/cairn -healthcheck` in scratch containers |
| `/sitemap.xml`, `/robots.txt` | for crawlers                                 |

Search is progressive enhancement: the full directory is server-rendered, a
small embedded script adds accent-insensitive filtering over names,
descriptions and tags. No JavaScript, no problem — the page just shows
everything.

## Documentation

The full tree lives in [docs/](docs/README.md): getting started,
configuration ([services](docs/configuration/services.md),
[site](docs/configuration/site.md), [theming](docs/configuration/theming.md),
[languages](docs/configuration/i18n.md)), hardened
[deployment](docs/deployment/docker-compose.md), recipes, an exhaustive
[reference](docs/reference.md), an honest [comparison](docs/comparison.md)
and a [FAQ](docs/faq.md).

## Roadmap

- **v0.3** — Gatus config emitter and optional status dots fed by a Gatus
  API URL, never by probing from the visitor's browser.

cairn is a directory, not a control panel: no auth, no widgets, no Docker
socket, no database. If you want admin widgets and integrations,
[Homepage](https://github.com/gethomepage/homepage) or
[Homer](https://github.com/bastienwirtz/homer) will fit you better.

## License

[GPL-3.0](LICENSE)
