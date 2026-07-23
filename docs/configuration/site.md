# Site

`site.yaml` describes everything around the cards. The file is optional, and
so is every key in it.

## Keys

| Key            | Default   | What it is                                        |
| -------------- | --------- | ------------------------------------------------- |
| `title`        | `cairn`   | Site name: hero heading, header, `<title>`.       |
| `tagline`      | empty     | One sentence under the title, translatable.       |
| `logo`         | none      | Image in the header: a URL or an [`/assets` path](../recipes/icons.md#your-own-files). |
| `locales`      | `[en]`    | Languages served. First entry is the default and the fallback; see [Languages](i18n.md). |
| `theme.accent` | `#247b7b` | Hex color for links, focus rings, buttons; see [Theming](theming.md). |
| `links`        | `[]`      | Quick links in the header, next to the language switcher. |
| `footer`       | `[]`      | Links at the bottom of every page.                |
| `pages`        | `[]`      | Pages cairn serves itself (legal notice, privacy…); see below. |
| `strings`      | built-ins | UI text overrides; see [Languages](i18n.md#ui-strings). |

## Full example

```yaml
title: Libre Internet
tagline:
  fr: Des outils libres, simples, pour tout le monde.
  en: Free, simple tools for everyone.
logo: /assets/logo.png
locales: [fr, en]
theme:
  accent: "#247b7b"
links:
  - label: Wiki
    url: https://wiki.example.org
  - label: GitHub
    url: https://github.com/you
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
  - label: Contact
    url: mailto:admin@example.org
pages:
  - id: legal
    title: { fr: Mentions légales, en: Legal notice }
    body:
      fr: |
        Éditeur : Prénom Nom, contact@exemple.org

        Hébergement : ce site est auto-hébergé par son éditeur.
      en: |
        Publisher: First Last, contact@example.org

        Hosting: this site is self-hosted by its publisher.
```

A typo in a key name is an error, not a silent no-op: cairn names the file
and the unknown key in the log.

## Hosted pages

Self-hosters rarely have another place to put a legal notice or a privacy
policy, so cairn serves these pages itself. Each entry in `pages` becomes
`/{locale}/{id}/`, rendered in the site layout: the translatable `title` as
the heading, the translatable `body` as plain-text paragraphs (blank line =
new paragraph, no markup). Every page is linked automatically at the end of
the footer, after your `footer` entries, in declaration order.

Page ids share the URL namespace with service ids; a collision is a config
error that names both sides.

Next: [Theming](theming.md)
