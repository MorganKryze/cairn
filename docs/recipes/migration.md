# Coming from Homer or Homepage

Your existing config already contains almost everything cairn needs, and
the icon slugs pass through unchanged (both projects use dashboard-icons).
Budget fifteen minutes for a handful of services.

## From Homer

Homer's `config.yml` groups items under `services`; each item maps directly:

| Homer                 | cairn                                             |
| --------------------- | ------------------------------------------------- |
| `services[].name`     | a `category` id, named in `categories.yaml`       |
| `items[].name`        | `name`                                            |
| `items[].subtitle`    | `desc`                                            |
| `items[].url`         | `url`                                             |
| `items[].logo` (slug) | `icon`, same slug                                 |
| `items[].tag`         | `tags`                                            |
| `title`, `subtitle`   | `site.yaml` `title`, `tagline`                    |
| `message`             | `site.yaml` `about`, now dismissable              |
| themes, colors        | `theme.accent`, then [custom.css](../configuration/theming.md) if you must |

```yaml
# Homer                                # cairn services.yaml
- name: Jellyfin                       - id: jellyfin
  logo: assets/icons/jellyfin.png        url: https://watch.example.org
  subtitle: Movies and shows             icon: jellyfin
  url: https://watch.example.org         name: Jellyfin
                                         desc: Movies and shows.
```

## From Homepage

Homepage's `services.yaml` is groups of `name: {href, description, icon}`:
the same three keys become `url`, `desc`, `icon`, the entry name becomes
`name` plus a lowercase `id`, and the group becomes `category`.

What has no equivalent, by design: widgets, API keys and Docker discovery
stay behind with the admin dashboard (keep it for yourself, they coexist
happily). `ping:`/`siteMonitor:` become a [Gatus](gatus.md) your server
polls; `cairn -emit-gatus` writes that config from your services.

While rewriting descriptions, switch voices: your old dashboard described
services for you, cairn's cards speak to your guests. "Jellyfin" tells
them nothing; "Movies and shows, no account needed" does.

Next: [Reference](../reference.md)
