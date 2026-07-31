# Theming

cairn ships one calm theme with automatic light and dark: it follows the
visitor's system preference, and a header toggle lets them force either.
The forced choice is remembered in their browser (localStorage, not a
cookie) and applies before first paint, so nothing flashes. You adjust the
theme itself in three steps of increasing depth; stop at the first one that
does the job.

## 1. Accent color

```yaml
# site.yaml
theme:
  accent: "#8a4f7d"
```

The accent colors links, focus rings, card hovers and the "Open the tool"
button. Body text stays neutral, so most accents are safe, but the button
sets white text on the accent, so pick a mid-to-dark accent (the default
`#247b7b` gives 5:1). A very light accent would weaken that button's
contrast.

## 2. custom.css

Drop a `custom.css` next to your YAML files; cairn serves it and loads it
after its own stylesheet, so your rules win. Live reload applies here too.

The stylesheet is built on custom properties; override those first:

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

| Variable         | Role                                                |
| ---------------- | --------------------------------------------------- |
| `--bg`           | page background                                     |
| `--fg`           | text                                                |
| `--muted`        | secondary text                                      |
| `--card`         | card, tile and search backgrounds                   |
| `--faint`        | icon tiles, subtle fills                            |
| `--border`       | card, chip and rule outlines                        |
| `--ui-border`    | the edge of a control you can operate               |
| `--accent`       | fills, focus rings, buttons (set by `theme.accent`) |
| `--accent-ink`   | accent tuned for text contrast (derived)            |
| `--up`, `--down` | status dot colors                                   |

Two border colors, because they answer to different rules. `--border` draws
decoration: the outline of a card, a rule, the trail's spine, all things the
page would still be legible without. `--ui-border` draws the edge of something
a visitor operates, the search box on the home page and the jump-to select a
phone gets, where nothing else says where to type or tap. That edge is held at
3:1 against the page and against its own fill, which is the floor
[WCAG asks for a control's boundary](https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html);
if you override it, keep it there.

Anything beyond variables is regular CSS on stable class names (`.card`,
`.cat`, `.way`, `.tile`, `.about`, `.menu`, `.toc`, `.detail`, `.btn`, …).
Keep [WCAG AA
contrast](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
in mind for text; the defaults pass it.

### Typography

Headings are set in [Fraunces](https://fonts.google.com/specimen/Fraunces), a
variable serif embedded in the binary and served locally, no external font
request. Body text uses the system UI font. Override `--font-display` or
`--font-body` in `custom.css` to change either.

## 3. Logo and favicon

```yaml
# site.yaml
logo: /assets/logo.png
```

Mount the file as described in [Icons: your own files](../recipes/icons.md#your-own-files).

Next: [Languages](i18n.md)
