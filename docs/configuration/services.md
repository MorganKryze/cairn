# Services

Services are the cards on the page. cairn reads them from every `*.yaml` file
in `/config` except `site.yaml` and `categories.yaml`, so you can keep one
file or [one file per category](../recipes/multiple-files.md).

Any text field accepts a plain string or a per-locale map; see
[Languages](i18n.md).

## Keys

| Key          | Required | Default       | What it is                                                                                                                                                   |
| ------------ | -------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `id`         | yes      | n/a           | Unique slug; becomes the detail page URL. Lowercase letters, digits, dashes.                                                                                 |
| `url`        | yes\*    | n/a           | Where the card links. `https://…` or an absolute path. Optional with `state: soon` or `state: retired`, which link nowhere.                                  |
| `name`       | yes      | n/a           | The card title.                                                                                                                                              |
| `desc`       | no       | empty         | One line under the name. Keep it plain: what the tool does for the visitor.                                                                                  |
| `details`    | no       | empty         | Longer text for the [detail page](#detail-pages); paragraphs and [CommonMark, minus raw HTML](text.md).                                                      |
| `images`     | no       | `[]`          | Screenshots for the detail page; see [Preview images](#preview-images).                                                                                      |
| `category`   | no       | `other`       | Group id; see `categories.yaml` in the [reference](../reference.md#categoriesyaml).                                                                          |
| `icon`       | no       | neutral glyph | Slug, URL or `/assets` path, or `{light, dark}` for a [monochrome mark](theming.md#artwork-that-cannot-serve-both-themes); see [Icons](../recipes/icons.md). |
| `tags`       | no       | `[]`          | Extra search words, invisible on the page. Add synonyms in every language.                                                                                   |
| `selfhosted` | no       | none          | `true` if you run it yourself, `false` if hosted elsewhere; shows a flag on the card. See [Hosting flag](#hosting-flag).                                     |
| `state`      | no       | none          | `soon`, `retired`, `beta`, `deprecated` or `new`; a badge on the card. See [Where a service stands](#where-a-service-stands).                                |

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
`details` text and an "Open" button. The card shows a discreet
detail-page glyph only when `details` or `images` is set; the card itself
always goes straight to the tool.

Write `details` for the visitor who wonders _"when would I use this?"_:
start from a situation, not from features.

## Hosting flag

Set `selfhosted: true` on a service you run yourself, or `selfhosted: false`
on one hosted by someone else. The card gains a small flag, bottom-left, that
shows an icon at rest and unfurls its label ("Self-hosted" / "External") on
hover or keyboard focus; on touch it stays open. Omit the key and the card
carries no flag.

It is a data-location cue for your guests: which tools keep their data on your
server, and which send them to a third party.

The flag is also what [`service_links.confirm: external`](site.md#leaving-the-site)
reads, if you want the ones you do not run to say so before a visitor follows
them. A service with no flag at all never counts as external there.

### Making the flag lead somewhere

By default the flag is text: a click on it opens the service, like the rest of
the card. Give it a target in `site.yaml` and it becomes a link of its own,
which is the place to explain what "self-hosted" means to a visitor who has
never met the word.

```yaml
# site.yaml
hosting_flag:
  self: hosting # a page cairn serves, by its id
  external: https://example.org/why # or any URL
```

Each key is optional and they are independent: set one and only that flag
becomes a link.

**The value is a page id, not a path.** cairn builds the link in the language
the visitor is reading, so a French visitor lands on the French page, the same
way the footer already links these pages. Writing `/en/hosting/` yourself would
pin one language for everybody. A path (`/why`) or a URL passes through as
written, and a URL opens in a new tab.

A name that matches no page stops the load and lists the pages that do exist.
`cairn -check` also says so when a target is set and no service carries that
flag, since the flag it points from is then never drawn.

One thing worth knowing before you set it: the card is one big link to the
service, and a linked flag is a second target inside it, next to the pill. On a
touch screen the flag is unfurled, so it is a wide one. That is the trade, and
it is why the flag stays plain text unless you ask.

The labels are translated in every built-in language and, like all UI text,
can be reworded per locale from `site.yaml`:

```yaml
strings:
  host.self: { fr: Chez moi, en: On my server }
  host.external: { fr: Cloud tiers, en: Third-party }
```

## Where a service stands

`state` puts a word on the card, above the name:

```yaml
- id: forge
  name: Forge
  state: soon
```

**The state is what you declare, the status is what the monitor measures.** They
sit on the same card and answer different questions: the pill in the corner
says whether the service answered a moment ago, the badge on the top edge says
whether it is worth going at all.

Five words, in two groups.

`beta`, `deprecated` and `new` change nothing but the badge. The link works, the
service is monitored as before, and its pill keeps reporting.

`soon` and `retired` take the destination away. The card stops being a link and
leaves the tab order, its status pill is gone since there is nothing to
monitor, and `-emit-gatus` writes no endpoint for it. The card is muted so it
reads as gone at a glance.

`url` becomes optional for those two: a service that does not exist yet has no
address, and one you retire keeps the address it had, in your file, no longer
linked. **Retiring a service is adding a line, never deleting one.**

A [detail page](#detail-pages) still works for both, and it is worth writing:
it is the one place left to say where people should go instead. It loses its
Open button and nothing else.

One honest word about `new`: it is the only state that goes stale on its own,
because nobody comes back to remove it. cairn will not chase it for you and
there is no date field to make it expire. That is deliberate: a directory that
needs a calendar is a different program.

The five words come from the strings table, so they read in the page's
language. See [Languages](i18n.md).

## Preview images

A screenshot answers _"what does it look like?"_ better than any paragraph.
Put the files in a `media/` folder next to your yaml and reference them by
name; each entry is a plain path or a `{src, caption}` pair:

```yaml
- id: photos
  # …
  images:
    - screen.png
    - src: albums.png
      caption: { fr: La vue albums., en: The albums view. }
```

They appear on the detail page, in order, between the text and the button.
cairn serves `media/` itself at `/media/`, so previews stay self-hosted like
everything else; a full URL or an `/assets/…` path passes through unchanged.

`screen.png` and `/media/screen.png` are two spellings of the same file, and
cairn opens the file behind either one before it serves the page. A name that
reaches nothing is a config error giving the path it looked for, so a typo
stops at startup rather than showing a visitor an empty frame.

Next: [Site](site.md)
