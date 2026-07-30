# Icons

`icon:` accepts three forms; cairn ships no icon pack of its own, and
services without an icon get a neutral glyph.

## Slugs (dashboard-icons)

```yaml
icon: stirling-pdf
```

A bare slug resolves against
[homarr-labs/dashboard-icons](https://github.com/homarr-labs/dashboard-icons),
the convention Homepage and Homarr already use, so configs migrate as-is.
Browse the catalog at <https://dashboardicons.com> to find slugs.

These icons load in the _visitor's_ browser from jsdelivr's CDN. The cairn
container itself never phones anywhere. If you want zero third-party
requests for your visitors too, use the `/assets` form below.

## URLs

```yaml
icon: https://cdn.jsdelivr.net/gh/selfhst/icons/svg/vaultwarden.svg
```

Any image URL works: the [selfh.st icon collection](https://selfh.st/icons/)
is a good second catalog.

## Your own files

Mount a directory at `/assets` and reference it:

```yaml
# compose.yaml
volumes:
  - ./config:/config:ro
  - ./assets:/assets:ro
```

```yaml
# services.yaml
icon: /assets/icons/intranet.svg
```

The same mount serves the site `logo:` and anything else you need
(`/assets/...` URLs map directly to the directory).

## Going fully self-hosted

Slug icons load from the jsdelivr CDN in your visitors' browsers: fine for
most sites, but it is the only third-party request on the page, and on a
network with no internet it means broken images. cairn removes it in two
steps and zero YAML edits:

```sh
docker run --rm -v ./config:/config ghcr.io/morgankryze/cairn:stable -emit-icons > get-icons.sh
cd assets && sh ../get-icons.sh
```

The script downloads each slug into `assets/icons/`. From then on cairn
resolves the slug to your local file automatically (an `icons/immich.svg`
next to your assets wins over the CDN, live, no restart needed), and
`cairn -check` stops warning about CDN icons.

That last part only holds if `-check` can see the folder, which means
`-assets` on the command line, or a second `-v` on a `docker run`. Given only
the config it keeps warning about icons you have already downloaded. On a
network with no route out, that check is the one that tells you whether you
are really ready: [Air-gapped](../deployment/airgap.md).

Next: [Multiple files](multiple-files.md)
