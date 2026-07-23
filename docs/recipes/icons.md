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

These icons load in the *visitor's* browser from jsdelivr's CDN. The cairn
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

Next: [Multiple files](multiple-files.md)
