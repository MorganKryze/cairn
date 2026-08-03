<div align="center">

# <sub><img src="src/internal/render/assets/favicon.svg" height="34" alt=""></sub> cairn

**The directory page for the people you host services _for_.**

[![Build](https://github.com/MorganKryze/cairn/actions/workflows/build.yml/badge.svg)](https://github.com/MorganKryze/cairn/actions/workflows/build.yml)
[![Security](https://github.com/MorganKryze/cairn/actions/workflows/security.yml/badge.svg)](https://github.com/MorganKryze/cairn/actions/workflows/security.yml)
[![Tests](https://raw.githubusercontent.com/MorganKryze/cairn/badges/tests.svg)](https://github.com/MorganKryze/cairn/actions/workflows/build.yml)
[![Coverage](https://raw.githubusercontent.com/MorganKryze/cairn/badges/coverage.svg)](https://github.com/MorganKryze/cairn/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/MorganKryze/cairn?label=release&color=247b7b)](https://github.com/MorganKryze/cairn/releases)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)

[![Image](https://img.shields.io/badge/image-4.4%20MB%20pull-2496ED?logo=docker&logoColor=white)](https://hub.docker.com/r/morgankryze/cairn/tags)
[![Docker Hub](https://img.shields.io/docker/pulls/morgankryze/cairn?label=docker%20hub&color=2496ED&logo=docker&logoColor=white&cacheSeconds=3600)](https://hub.docker.com/r/morgankryze/cairn)
[![Go](https://img.shields.io/badge/Go-single%20static%20binary-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Helm](https://img.shields.io/badge/Helm-chart-0F1689?logo=helm&logoColor=white)](docs/deployment/helm.md)
[![Docker Compose](https://img.shields.io/badge/Compose-ready-2496ED?logo=docker&logoColor=white)](docs/deployment/docker-compose.md)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-ready-326CE5?logo=kubernetes&logoColor=white)](docs/deployment/kubernetes.md)
[![Context7](https://img.shields.io/badge/Context7-ask%20your%20assistant-247b7b?logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0iI2ZmZiI+PHBhdGggZD0iTTEyIDJhMTAgMTAgMCAxIDAgMCAyMCAxMCAxMCAwIDAgMCAwLTIwem0wIDNhNyA3IDAgMSAxIDAgMTQgNyA3IDAgMCAxIDAtMTR6Ii8+PC9zdmc+)](https://context7.com/morgankryze/cairn)

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

## Features

### What your visitors get

- 👋 **A welcome note in your words**: who hosts this, for whom, how to
  reach you. Dismissable, remembered for a year (a plain cookie, no
  tracking).
- 🗂️ **Tools grouped by need**, one plain sentence each, with optional
  "Learn more" pages for the curious.
- 🚦 **Live status pills** fed by the monitor you already run, whichever it
  is, each linking to its status page. Your server does the polling, never
  the visitor's browser.
- 🌍 **Their language**: the server picks it from the browser, a switcher
  pins it. One config file, translations inline.
- 🔍 **Search from anywhere**: just start typing, or ⌘K. A name finds that
  one service, not everything that mentions it; keyword search still reaches
  descriptions and hidden tags, accent-insensitive.
- 📱 **At home on a phone**: the layout reflows to one hand, the header steps
  aside as you scroll and returns the moment you head back up, and a burger
  keeps search within thumb's reach.
- ♿ **Built to stay readable**: text contrast measured against WCAG AA in both
  themes, a skip link, named landmarks, search results announced through a
  live region, a theme toggle that says which way it is set, and
  right-to-left languages laid out the right way round. The controls a
  visitor can operate, the search box and the phone's jump-to select, carry a
  boundary that clears 3:1, remeasured in a real browser on every pull
  request. Tested with a browser, never audited as a whole, and not yet with a
  real screen reader: if you use one,
  [tell us what breaks](https://github.com/MorganKryze/cairn/issues).
- 🌗 **Calm typography, light and dark, keyboard-friendly**, and every
  feature degrades gracefully without JavaScript.

<div align="center">

<a href="https://cairn.libresoftware.cloud" title="Open the live demo"><img src="docs/assets/two-views.png" alt="Left: a service detail page, with the name, a live status pill, a paragraph explaining what the tool is for, a screenshot with its caption and a button opening the tool. Right: the same directory on a phone, category chips then cards each showing an icon, a sentence, a self-hosted flag and a status pill" width="100%"></a>

<em>Behind a card when a visitor wants more than one sentence, and the same page in a hand.</em>

</div>

### What you get as the operator

- 📦 **One static binary, about 10 MB**, zero runtime dependencies, no
  database. The image around it is 4.4 MB to pull on amd64 and 4.0 on arm64,
  since that binary compresses well.
- 📝 **YAML mounted read-only**; edits apply live within seconds, and a config
  error names the file, the line and the expected shape instead of taking the
  site down.
- 📜 **Legal pages served by cairn itself** (legal notice, privacy…): the
  pages self-hosters never have anywhere to put. Written in ordinary
  [markdown](docs/configuration/text.md), headings and lists and tables, with
  raw HTML left as text so a config file can never reach the page as markup.
- 🌐 **Wherever your domain has room**: a domain, a subdomain, or a sub-path of
  one you already use (`example.org/cairn/`, one flag). cairn handles the
  prefix itself, so the [proxy](docs/deployment/reverse-proxies.md) needs no
  rewriting rule.

## Status monitoring

**Whatever already tells you it is up.** cairn does not ask you to change monitors. It reads the one you run, and the
pills come from your server, never from the visitor's browser.

<table>
<tr><td><strong>Self-hosted</strong></td><td>
<a href="https://github.com/TwiN/gatus">Gatus</a> ·
<a href="https://github.com/louislam/uptime-kuma">Uptime Kuma</a> ·
<a href="https://github.com/cachethq/cachet">Cachet</a> ·
<a href="https://github.com/statping-ng/statping-ng">Statping-ng</a> ·
<a href="https://upptime.js.org">Upptime</a>
</td></tr>
<tr><td><strong>Hosted</strong></td><td>
<a href="https://www.atlassian.com/software/statuspage">Atlassian Statuspage</a> ·
<a href="https://instatus.com">Instatus</a> ·
<a href="https://uptimerobot.com">UptimeRobot</a> ·
<a href="https://betterstack.com/uptime">Better Stack</a> ·
<a href="https://www.statuscake.com">StatusCake</a>
</td></tr>
</table>

Every one of those was read from a live instance rather than from a manual, and
the mapping each needed is written down with the count that came back. Anything
else publishing a list of names and states takes six lines of configuration:
[which monitors cairn reads](docs/recipes/status.md#which-monitors-cairn-reads)
also says what cannot be read, and why.

<table>
<tr>
<td width="86" align="center" valign="middle">
<a href="https://github.com/TwiN/gatus"><img src="https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/gatus.svg" width="56" alt="Gatus"></a>
</td>
<td valign="middle">

**[Gatus](https://github.com/TwiN/gatus) gets the warmest handshake, and has earned it.**
It is the only one cairn integrates with **both ways**, since `cairn -emit-gatus`
writes its endpoint config out of your services, and the only one whose pills
link to a page per service rather than to one page for everything. If you have
no monitor yet, start there.

</td>
</tr>
</table>

## Security

**Secure by subtraction:** the safest surface is the one that isn't there. cairn stores nothing, signs no
one in, and takes no input it has to trust. What little runs is kept
deliberately tight, from the image down to the wire.

- 🪨 **`FROM scratch`, non-root**: no shell, no package manager, no libc in
  the image, so a compromised process has nothing to pivot into.
- 🛡️ **Runs locked down**: `read_only`, `cap_drop: ALL` and a self-probing
  healthcheck all work out of the box. See the [hardened compose](docs/deployment/docker-compose.md).
- 🧱 **A strict Content-Security-Policy** (`default-src 'none'`, inline
  fragments pinned by hash) and the hardening headers, with no third-party
  script or font to trust.
- 🔌 **No outbound requests of its own**: air-gap friendly, its only companions
  optional and yours, a self-hosted [monitor](docs/recipes/status.md) for status
  and icons you can [serve yourself](docs/recipes/icons.md#going-fully-self-hosted).
  The [demo](demo/README.md) runs on a network with no route out and serves its
  own icons, so the page it shows you makes no third-party request at all.
- 🔬 **A watched supply chain**: every pull request runs `govulncheck`, a Trivy
  image scan and CodeQL, and they run again on the pushes that change code and
  weekly on a schedule. Dependabot follows the dependencies, and every CI action
  is pinned to a commit rather than a movable tag, as is the buildkit image that
  runs the build.
- 🧪 **More test than product**: about 8 800 lines of tests against 6 000 of
  code, 505 of them run on every pull request, and 92.9% of the statements are
  covered with an 87% floor that fails the build. A browser suite drives what
  markup cannot show, and every one of those tests was made to fail before the
  fix it covers, by patching the fix out and reading the red.
- ✍️ **Artifacts you can check**: the image is signed with cosign and ships SLSA
  provenance and an SBOM; the release binaries carry their own attestation.
  The [verification commands](SECURITY.md#verifying-what-you-pulled) are two
  lines.

## Quickstart

Nothing to write, nothing to mount, just look at it:

```sh
docker run --rm -p 8080:8080 morgankryze/cairn:stable
```

That is a running cairn on <http://localhost:8080>, telling you what to feed
it. When you are ready to feed it, two files and one command:

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
    image: morgankryze/cairn:latest
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

Everything lives in [docs/](docs/README.md), and it is written to be read, not
just grepped: each page teaches the why before the how, in plain prose a
developer can enjoy rather than a reference dump to endure. If you like
understanding how a thing works, start anywhere below.

<table>
<tr><td><strong>Start</strong></td><td>
<a href="docs/getting-started.md">Getting started</a>, the five-minute path ·
<a href="docs/upgrading.md">Upgrading</a>, what a new version can refuse and how to fix it
</td></tr>
<tr><td><strong>Configure</strong></td><td>
<a href="docs/configuration/services.md">Services</a> ·
<a href="docs/configuration/site.md">Site</a> ·
<a href="docs/configuration/text.md">Text</a> ·
<a href="docs/configuration/theming.md">Theming</a> ·
<a href="docs/configuration/i18n.md">Languages</a>
</td></tr>
<tr><td><strong>Deploy</strong></td><td>
<a href="docs/deployment/docker-compose.md">Docker Compose</a> ·
<a href="docs/deployment/podman.md">Podman</a> ·
<a href="docs/deployment/binary.md">Bare binary</a> ·
<a href="docs/deployment/kubernetes.md">Kubernetes</a> ·
<a href="docs/deployment/helm.md">Helm</a> ·
<a href="docs/deployment/airgap.md">Air-gapped</a> ·
<a href="docs/deployment/reverse-proxies.md">Reverse proxies</a>
</td></tr>
<tr><td><strong>Recipes</strong></td><td>
<a href="docs/recipes/status.md">Status page</a> ·
<a href="docs/recipes/icons.md">Icons</a> ·
<a href="docs/recipes/multiple-files.md">Multiple files</a> ·
<a href="docs/recipes/migration.md">Migration</a>
</td></tr>
<tr><td><strong>Look up</strong></td><td>
<a href="docs/reference.md">Reference</a> ·
<a href="docs/faq.md">FAQ</a> ·
<a href="docs/comparison.md">Comparison</a>
</td></tr>
</table>

### For your AI assistant

Those pages are indexed on
**[Context7](https://context7.com/morgankryze/cairn)**, so an assistant with
the Context7 MCP server can pull cairn's real documentation instead of
inventing config keys that never existed. Ask it for `morgankryze/cairn` by
name.

## Scope

**Not a dashboard, on purpose.** cairn is a directory, not a control panel: no auth, no widgets, no Docker
socket, no admin UI. If the audience is _you_, the admin,
[Homepage](https://github.com/gethomepage/homepage) or
[Homer](https://github.com/bastienwirtz/homer) will make you happier; the
[comparison](docs/comparison.md) is honest about it.

## Contributing

cairn is young and opinionated, and other people's eyes make it better.
Ideas and bug reports are welcome in the
[issues](https://github.com/MorganKryze/cairn/issues), code and docs through
[contributing](CONTRIBUTING.md), always within the scope above. And if cairn
serves your people well, a [coffee](https://ko-fi.com/morgankryze) keeps its
maintainer walking the trail.

## Contributors

Following the [all-contributors](https://allcontributors.org) convention,
which counts every kind of contribution rather than only the commits: an
issue that names a real problem, a bug report with the screenshot that cracks
it, and a translation are all work.

<table>
<tr>
<td align="center" width="150">
<a href="https://github.com/MorganKryze"><img src="https://github.com/MorganKryze.png?size=100" width="80" alt=""><br><sub><b>MorganKryze</b></sub></a><br>
<a href="https://github.com/MorganKryze/cairn/commits?author=MorganKryze" title="Code">💻</a>
<a href="https://github.com/MorganKryze/cairn/tree/main/docs" title="Documentation">📖</a>
<span title="Design">🎨</span>
<span title="Ideas and planning">🤔</span>
<span title="Maintenance">🚧</span>
</td>
<td align="center" width="150">
<a href="https://github.com/AntonPalmqvist"><img src="https://github.com/AntonPalmqvist.png?size=100" width="80" alt=""><br><sub><b>AntonPalmqvist</b></sub></a><br>
<a href="https://github.com/MorganKryze/cairn/issues/33" title="Bug reports">🐛</a>
<a href="https://github.com/MorganKryze/cairn/issues/33" title="Ideas and planning">🤔</a>
</td>
<td align="center" width="150">
<a href="https://github.com/rbourgeat"><img src="https://github.com/rbourgeat.png?size=100" width="80" alt=""><br><sub><b>rbourgeat</b></sub></a><br>
<a href="https://github.com/MorganKryze/cairn/pull/51" title="Code">💻</a>
<a href="https://github.com/MorganKryze/cairn/issues/50" title="Ideas and planning">🤔</a>
</td>
</tr>
</table>

💻 code · 📖 documentation · 🎨 design · 🤔 ideas · 🐛 bug reports · 🚧 maintenance

The avatars come from GitHub itself rather than from a third-party image
service, for the reason the coverage badge is self-hosted: nothing about this
repository should depend on somebody else's uptime to render. Adding yourself
here is part of a pull request, not an afterthought.

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
