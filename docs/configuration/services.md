# Services

Services are the cards on the page. cairn reads them from every `*.yaml` file
in `/config` except `site.yaml` and `categories.yaml`, so you can keep one
file or [one file per category](../recipes/multiple-files.md).

Any text field accepts a plain string or a per-locale map; see
[Languages](i18n.md).

## Keys

| Key        | Required | Default       | What it is                                |
| ---------- | -------- | ------------- | ----------------------------------------- |
| `id`       | yes      | n/a           | Unique slug; becomes the detail page URL. Lowercase letters, digits, dashes. |
| `url`      | yes      | n/a           | Where the card links. `https://…` or an absolute path. |
| `name`     | yes      | n/a           | The card title.                           |
| `desc`     | no       | empty         | One line under the name. Keep it plain: what the tool does for the visitor. |
| `details`  | no       | empty         | Longer text for the [detail page](#detail-pages). Blank lines split paragraphs. |
| `category` | no       | `other`       | Group id; see `categories.yaml` in the [reference](../reference.md#categoriesyaml). |
| `icon`     | no       | neutral glyph | Slug, URL or `/assets` path; see [Icons](../recipes/icons.md). |
| `tags`     | no       | `[]`          | Extra search words, invisible on the page. Add synonyms in every language. |

## Full example

```yaml
- id: photos
  url: https://photos.example.org
  category: photos
  icon: immich
  name: { fr: Photothèque, en: Photo library }
  desc: { fr: Sauvegarder et retrouver vos photos., en: Back up and browse your photos. }
  details:
    fr: |
      Vos photos de téléphone, sauvegardées automatiquement et retrouvables
      par date, par lieu ou par visage.
    en: |
      Your phone photos, backed up automatically and searchable by date,
      place or face.
  tags: [images, backup, sauvegarde]
```

## Detail pages

Every service gets a page at `/{locale}/{id}/` with its name, description,
`details` text and an "Open the tool" button. The card shows a discreet
"Learn more" link only when `details` is set; the card itself always goes
straight to the tool.

Write `details` for the visitor who wonders *"when would I use this?"*:
start from a situation, not from features.

Next: [Site](site.md)
