# Upgrading

cairn reads your config at every start and on every change. When a version
tightens what it accepts, a file that loaded yesterday can be refused today,
and this page is where each of those refusals is written down with its fix.

## Check before you switch

The new image validates your existing config without touching anything you are
running. Do this first, every time:

```sh
docker run --rm -v ./config:/config ghcr.io/morgankryze/cairn:1.13.1 -check
```

Exit code 0 means the upgrade will load. Exit 1 prints the exact message and
the file it came from, and the sections below say what to do about it. Add
`-v ./assets:/assets` if you mount one; some checks open files and stay silent
without it.

## What happens if you skip it

cairn never exits on a bad config, but the three deployments differ, and it is
worth knowing which one you are in.

- **A running instance that reloads.** It keeps the last good pages and stays
  ready. The log names the problem. Nothing a visitor sees changes.
- **Kubernetes, with the chart or the manifests.** The new pod loads the
  config, fails, and `/readyz` stays 503. The rolling update never completes,
  so the old pod keeps serving. You get a stuck rollout, not a broken site.
- **Docker Compose, Podman, or the bare binary, restarted.** `/healthz` is
  liveness and stays 200, because a process serving the getting-started page is
  not one to kill. So the container comes back up healthy and **serves the
  getting-started page instead of your site**, with the reason in the log.

That last one is the case the dry run above exists for.

## 1.13.0

Seven shapes that used to load are now refused. Every one of them was a value
that did nothing: not one page that renders correctly today stops rendering.
What changed is that a silent defect became a named error.

Each heading is the message you will find in your log.

### `url … must be the domain alone, with no path`

```console
config: site.yaml: url "https://example.org/cairn" must be the domain alone, with no path (serving under example.org/cairn is what -base-path is for, and cairn adds that prefix itself)
```

You are serving under a sub-path and wrote the full public address. cairn
appends `-base-path` itself, so the prefix was counted twice: canonical links,
hreflang alternates and the whole sitemap pointed at `example.org/cairn/cairn/`,
and search engines acted on it.

```yaml
# site.yaml
url: https://example.org # the domain alone
```

The prefix stays where it belongs, on the flag:

```sh
cairn -base-path /cairn
```

More in [Reverse proxies](deployment/reverse-proxies.md#under-a-sub-path).

### `uses the tel: scheme, which cairn does not emit`

```console
config: site.yaml: footer url "tel:+33123456789" uses the tel: scheme, which cairn does not emit: the link would render dead and nothing would say why (expected https://…, mailto:… or an absolute path)
```

`html/template` writes only `http`, `https`, `mailto` and paths into an `href`,
and replaces every other scheme with a placeholder that goes nowhere. Your
`tel:` link has always rendered, looked live, and done nothing when clicked.

Put the number where it can be read and point the link at a page:

```yaml
footer:
  - label: "Support: +33 1 23 45 67 89"
    url: /contact
```

A hosted page is one entry in `pages`; see
[Hosted pages](configuration/site.md#hosted-pages). The same rule refuses
`javascript:`, `vbscript:` and `data:`, which are the three a browser can be
talked into executing. `security.contact` is the exception and still takes
`tel:`, because that file is plain text rather than markup. Full reasoning in
[Links a browser will not follow](configuration/site.md#links-a-browser-will-not-follow).

### `footer entry … has an icon`

```console
config: site.yaml: footer entry "https://status.example.org" has an icon: only header links render one (move the entry to links, or drop the icon)
```

The footer is a row of plain text links by design. The key was accepted and
then dropped, `cairn -check` reported it as inert, and `schema/site.json` never
listed it at all, so an editor with the schema wired up underlined it while
cairn started happily.

Drop the key, or move the entry to `links`, where the icon does work:

```yaml
links:
  - { label: Status, url: https://status.example.org, icon: status }
```

### `every footer entry needs label and url`

```console
config: site.yaml: every footer entry needs label and url (expected: - {label: Legal, url: /legal})
```

`links` has always demanded both. An entry with no `url` rendered an empty
`href`, which is a link that looks live and silently reloads the page the
visitor is already on. One with no `label` rendered as an anchor with no text,
which a screen reader announces as a link and then reads out its address.

### `icon … is not a slug, a URL or an /assets path`

```console
config: services.yaml line 12: service "pad": icon "HedgeDoc" is not a slug, a URL or an /assets path (a slug is lowercase letters, digits, dashes, e.g. hedgedoc)
```

A slug is what [dashboard-icons](https://github.com/homarr-labs/dashboard-icons)
publishes: lowercase letters, digits and dashes. Anything else could never
resolve there, so the card was already showing a broken image.

```yaml
- id: pad
  icon: hedgedoc # a slug
  icon: /assets/icons/pad.svg # or your own file
  icon: https://example.org/pad.png # or a URL
```

The rule is strict rather than forgiving because that value becomes a filename
in the script `-emit-icons` writes, and the documentation tells you to pipe
that script into `sh`. See [Icons](recipes/icons.md).

### `theme.accent … is not a hex color`

```console
config: site.yaml: theme.accent "#12345" is not a hex color (expected e.g. "#247b7b")
```

CSS hex colors are 3, 4, 6 or 8 digits. Five and seven used to pass, and an
invalid custom property makes every `var(--accent)` declaration invalid at
computed-value time: your accent, and every focus ring drawn with it, silently
disappeared.

Count the digits, and quote the value so YAML does not read the `#` as a
comment.

### `image … not found`

```console
config: service "pad": image "/media/shot.png" not found (expected a file at /config/media/shot.png, a URL or an absolute path)
```

`shot.png` and `/media/shot.png` name the same file and the documentation
presents them as two spellings of one thing. Only the first was ever opened, so
the same missing file refused to boot written one way and rendered a broken
image written the other. Add the file, or drop the entry.

## Nothing else in 1.13.0 needs your attention

The rest of the release changes behaviour without asking anything of your
config: `/.well-known/security.txt` if you set `security.contact`, gzip on the
text responses, an `ETag` that names its encoding, security headers on the root
probes under `-base-path`, dot-prefixed paths refused under `-assets` and
`media/`, `lang` and `dir` on text that fell back to another language, a
stronger card outline, and a longer list of `cairn -check` warnings.

If you deliberately mounted a dot-prefixed file under `-assets`, that one path
now returns 404. It is there so that pointing `-assets` at a working copy stops
publishing `.git/config`. Rename the file.
