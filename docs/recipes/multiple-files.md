# Multiple files

One `services.yaml` is fine until it isn't. cairn treats **every** `*.yaml`
or `*.yml` in `/config` as a list of services, except `site.yaml` and
`categories.yaml`, which keep their special roles. So one file per category
works with no extra config:

```text
config/
  site.yaml
  categories.yaml
  documents.yaml
  media.yaml
  admin-tools.yaml
```

```yaml
# documents.yaml
- id: pdf
  url: https://pdf.example.org
  category: documents
  name: { fr: Boîte à outils PDF, en: PDF toolbox }

- id: pad
  url: https://pad.example.org
  category: documents
  name: { fr: Bloc-notes partagé, en: Shared notepad }
```

The rules:

- File names don't matter (beyond the two reserved ones) and carry no
  meaning: the `category` key decides the grouping, one file can feed
  several categories.
- Files merge in name order; services keep the order of their file. Explicit
  ordering of the _groups_ lives in `categories.yaml`; see the
  [reference](../reference.md#categoriesyaml).
- A service `id` must be unique across all files. A duplicate is a config
  error naming both files, not a silent override.

Next: [Status page](gatus.md)
