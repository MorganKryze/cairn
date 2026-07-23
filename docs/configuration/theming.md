# Theming

cairn ships one calm theme with automatic light and dark (it follows the
visitor's system preference). You adjust it in three steps of increasing
depth; stop at the first one that does the job.

## 1. Accent color

```yaml
# site.yaml
theme:
  accent: "#8a4f7d"
```

The accent colors links, focus rings, card hovers and the "Open the tool"
button. Text stays neutral for contrast — pick any accent without breaking
readability.

## 2. custom.css

Drop a `custom.css` next to your YAML files; cairn serves it and loads it
after its own stylesheet, so your rules win. Live reload applies here too.

The stylesheet is built on custom properties — override those first:

```css
/* /config/custom.css */
:root {
  --bg: #fdf6ec;
  --card: #fffdf8;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #101418;
  }
}
```

| Variable   | Role                        |
| ---------- | --------------------------- |
| `--bg`     | page background             |
| `--fg`     | text                        |
| `--muted`  | secondary text              |
| `--card`   | card and input background   |
| `--border` | card and input borders      |
| `--accent` | links, focus, buttons       |

Anything beyond variables is regular CSS on stable class names (`.hero`,
`.card`, `.cat`, `.detail`, `.btn`, …). Keep [WCAG AA
contrast](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
in mind — the defaults pass it.

## 3. Logo and favicon

```yaml
# site.yaml
logo: /assets/logo.png
```

Mount the file as described in [Icons — your own files](../recipes/icons.md#your-own-files).

Next: [Languages](i18n.md)
