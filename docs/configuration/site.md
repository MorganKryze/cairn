# Site

`site.yaml` describes everything around the cards. The file is optional, and
so is every key in it.

## Keys

| Key            | Default   | What it is                                        |
| -------------- | --------- | ------------------------------------------------- |
| `title`        | `cairn`   | Site name: hero heading, header, `<title>`.       |
| `tagline`      | empty     | One sentence under the title, translatable.       |
| `logo`         | none      | Image in the header: a URL or an [`/assets` path](../recipes/icons.md#your-own-files). |
| `locales`      | `[en]`    | Languages served. First entry is the default and the fallback — see [Languages](i18n.md). |
| `theme.accent` | `#247b7b` | Hex color for links, focus rings, buttons — see [Theming](theming.md). |
| `footer`       | `[]`      | Links at the bottom of every page.                |
| `strings`      | built-ins | UI text overrides — see [Languages](i18n.md#ui-strings). |

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
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
  - label: Contact
    url: mailto:admin@example.org
```

A typo in a key name is an error, not a silent no-op — cairn names the file
and the unknown key in the log.

Next: [Theming](theming.md)
