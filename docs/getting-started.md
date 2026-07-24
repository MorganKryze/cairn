# Getting started

Five minutes, copy-paste, no prior knowledge of cairn.

## 1. Create the config

```sh
mkdir -p cairn/config && cd cairn
```

Save this as `config/services.yaml`:

```yaml
- id: pdf
  url: https://pdf.example.org
  category: documents
  icon: stirling-pdf
  name: PDF toolbox
  desc: Merge, split, compress your PDFs.
```

Replace the URL with one of your services. This one file is enough. The
`category` id becomes a group heading on the page (capitalized, dashes
turned into spaces); you
can name and order groups later with `categories.yaml`. And every text key
also accepts a per-locale map (`name: { fr: …, en: … }`); see
[Languages](configuration/i18n.md).

## 2. Run it

Save this as `compose.yaml`, next to the `config` folder:

```yaml
services:
  cairn:
    image: ghcr.io/morgankryze/cairn:latest
    ports:
      - 8080:8080
    volumes:
      - ./config:/config:ro
    read_only: true
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    healthcheck:
      test: ["CMD", "/cairn", "-healthcheck"]
      interval: 30s
```

```sh
docker compose up -d
```

Open <http://localhost:8080>. You get a finished page: one category, one
card, working search, light and dark.

## 3. Make it yours

Add `config/site.yaml`, every key optional:

```yaml
title: Libre Internet
tagline:
  fr: Des outils libres, simples, pour tout le monde.
  en: Free, simple tools for everyone.
locales: [fr, en]
```

Save the file: the page updates within a couple of seconds, no restart.
Config mistakes never take the site down: the previous config keeps serving
and the log (`docker compose logs -f cairn`) tells you the file, the line
and what was expected.

## Two safety nets while you write

Put this line at the top of a yaml file and any editor with a YAML language
server (VS Code with the YAML extension, for instance) autocompletes every
key, documents it on hover, and underlines mistakes as you type:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/MorganKryze/cairn/main/schema/services.json
```

Use `schema/site.json` for `site.yaml` and `schema/categories.json` for
`categories.yaml`.

And `cairn -check` validates a config directory without serving anything,
then warns about the likely oversights (a translation missing in one
locale, a `media/` file nothing references, an image heavy enough to hurt).
It exits 0 or 1, so it slots into CI if you version your config:

```sh
docker run --rm -v ./config:/config ghcr.io/morgankryze/cairn:stable -check
```

## Next

- Add more cards: [Services](configuration/services.md)
- Welcome your visitors and link your world: `about`, `links` and hosted
  legal pages in [Site](configuration/site.md)
- Name and order your groups: the `categories.yaml` section of the [reference](reference.md#categoriesyaml)
- Show live status on the cards: [Status page with Gatus](recipes/gatus.md)
- Put it behind your domain: [Reverse proxies](deployment/reverse-proxies.md)
