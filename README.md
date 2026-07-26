<div align="center">

# <sub><img src="src/assets/favicon.svg" height="34" alt=""></sub> cairn

**The directory page for the people you host services _for_.**

[![Build](https://github.com/MorganKryze/cairn/actions/workflows/build.yml/badge.svg)](https://github.com/MorganKryze/cairn/actions/workflows/build.yml)
[![Security](https://github.com/MorganKryze/cairn/actions/workflows/security.yml/badge.svg)](https://github.com/MorganKryze/cairn/actions/workflows/security.yml)
[![Coverage](https://raw.githubusercontent.com/MorganKryze/cairn/badges/coverage.svg)](https://github.com/MorganKryze/cairn/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/MorganKryze/cairn?label=release&color=247b7b)](https://github.com/MorganKryze/cairn/releases)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-single%20static%20binary-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Image](https://img.shields.io/badge/image-FROM%20scratch%20·%20~14%20MB-2496ED?logo=docker&logoColor=white)](https://github.com/MorganKryze/cairn/pkgs/container/cairn)
[![Self-hosted](https://img.shields.io/badge/self--hosted-yes-ff69b4)](docs/deployment/docker-compose.md)

</div>

> A cairn is a small stack of stones left by hikers who walked the trail
> before you, so you find your way without digging.

<div align="center">

<a href="https://cairn.libresoftware.cloud" title="Open the live demo">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/home-dark.png">
  <img src="docs/assets/home-light.png" alt="cairn home page: a welcome note, service cards grouped by category with live status pills, and a category trail in the margin" width="100%">
</picture>
</a>

</div>

---

Your family, your clients, your friends: they don't want a dashboard, they
want to know what this place is, what each tool does, and whether it works
right now. cairn is that page. Written in their language, readable without an
account or a manual, and boring for you to operate.

## What your visitors get

- **A welcome note in your words**: who hosts this, for whom, how to reach
  you. Dismissable, remembered for a year (a plain cookie, no tracking).
- **Tools grouped by need**, one plain sentence each, with optional "Learn
  more" pages for the curious.
- **Live status pills** fed by your [Gatus](https://github.com/TwiN/gatus),
  each linking to its own endpoint page. Your server does the polling,
  never the visitor's browser.
- **Their language**: the server picks it from the browser, a switcher pins
  it. One config file, translations inline.
- **Search from anywhere**: just start typing, or ⌘K. Works accent-insensitive
  over names, descriptions and hidden tags.
- **Calm typography, light and dark, keyboard-friendly**, and every feature
  degrades gracefully without JavaScript.

## What you get as the operator

- **One binary, `FROM scratch`, ~14 MB**, zero runtime dependencies, no
  database, no tracking.
- **YAML mounted read-only**; edits apply live within seconds, and a config
  error names the file, the line and the expected shape instead of taking the
  site down.
- **Legal pages served by cairn itself** (legal notice, privacy…): the pages
  self-hosters never have anywhere to put.
- The strictest container settings just work: `read_only`, `cap_drop: ALL`,
  self-probing healthcheck. See the [hardened compose](docs/deployment/docker-compose.md).
- **Air-gap friendly**: cairn makes zero outbound requests of its own. Its
  only companions are optional and yours: a self-hosted
  [Gatus](docs/recipes/gatus.md) for status, and icons you can
  [serve yourself](docs/recipes/icons.md#going-fully-self-hosted). The
  [demo](demo/README.md) runs on an internal network with no route out, to
  prove it.

## Secure by subtraction

The safest surface is the one that isn't there: cairn has no database, no
accounts, no admin panel, no Docker socket, and no form to submit. What little
remains is kept deliberately tight.

- **`FROM scratch`, non-root**: no shell, no package manager, no libc in the
  image, so a compromised process has nothing to pivot into.
- **A strict Content-Security-Policy** (`default-src 'none'`, inline fragments
  pinned by hash) and the hardening headers, with no third-party script or
  font to trust.
- **A watched supply chain**: every push runs `govulncheck`, a Trivy image
  scan, CodeQL and Dependabot. See [security.yml](.github/workflows/security.yml).

## Quickstart

Two files, one command:

```yaml
# config/services.yaml
- id: pdf
  url: https://pdf.example.org
  icon: stirling-pdf
  name: PDF toolbox
  desc: Merge, split, compress your PDFs.
```

(Want two languages? `name: { fr: Boîte à outils PDF, en: PDF toolbox }`;
every text key works both ways. See [Languages](docs/configuration/i18n.md).)

```yaml
# compose.yaml
services:
  cairn:
    image: ghcr.io/morgankryze/cairn:latest
    ports:
      - 8080:8080
    volumes:
      - ./config:/config:ro
```

```sh
docker compose up -d
```

Open <http://localhost:8080>: that is a finished page. Everything else
(title, languages, categories, status, theming) is one optional key at a
time, at your pace: follow [Getting started](docs/getting-started.md).

Prefer to see it live first? A public instance runs at
**<https://cairn.libresoftware.cloud>**, and the [demo stack](demo/README.md)
spins up your own copy, with a real Gatus and a handful of sample services,
one intentionally dead, in one command:

```sh
git clone https://github.com/MorganKryze/cairn.git && cd cairn/demo
docker compose up -d --build
```

## Documentation

Everything lives in [docs/](docs/README.md):

|               |                                                                                                                                                                      |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Start**     | [Getting started](docs/getting-started.md), the five-minute path                                                                                                     |
| **Configure** | [Services](docs/configuration/services.md) · [Site](docs/configuration/site.md) · [Text](docs/configuration/text.md) · [Theming](docs/configuration/theming.md) · [Languages](docs/configuration/i18n.md) |
| **Deploy**    | [Docker Compose](docs/deployment/docker-compose.md) · [Podman](docs/deployment/podman.md) · [Bare binary](docs/deployment/binary.md) · [Reverse proxies](docs/deployment/reverse-proxies.md)                                                          |
| **Recipes**   | [Status page with Gatus](docs/recipes/gatus.md) · [Icons](docs/recipes/icons.md) · [Multiple files](docs/recipes/multiple-files.md) · [Migration](docs/recipes/migration.md)                                  |
| **Look up**   | [Reference](docs/reference.md) · [FAQ](docs/faq.md) · [Comparison](docs/comparison.md)                                                                               |

## Not a dashboard, on purpose

cairn is a directory, not a control panel: no auth, no widgets, no Docker
socket, no admin UI. If the audience is _you_, the admin,
[Homepage](https://github.com/gethomepage/homepage) or
[Homer](https://github.com/bastienwirtz/homer) will make you happier; the
[comparison](docs/comparison.md) is honest about it.

## Ideas, contributions, support

cairn is young and opinionated, and other people's eyes make it better.
Ideas and bug reports are welcome in the
[issues](https://github.com/MorganKryze/cairn/issues), code and docs through
[contributing](CONTRIBUTING.md), always within the scope above. And if cairn
serves your people well, a [coffee](https://ko-fi.com/morgankryze) keeps its
maintainer walking the trail.

## Colophon

A colophon tells how the book was made, so here is mine: Go, plain YAML,
[Fraunces](https://github.com/undercasetype/Fraunces) for the headings, and
[Claude Code](https://claude.com/claude-code) drafting at my side, never on
autopilot. The taste, the reviews and the final word stay mine; the tests,
the CI and the public history keep me honest.

## License

Free software under [GPL-3.0](LICENSE): use it, modify it, share it. What
you redistribute stays under the same license, source included. Hosting your
own instance is not distribution and asks nothing of you.
