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

The accent colors links, focus rings, card hovers and the "Open"
button. Body text stays neutral, so most accents are safe, but the button
sets white text on the accent, so pick a mid-to-dark accent (the default
`#247b7b` gives 5:1). A very light accent would weaken that button's
contrast.

## 2. Typography

```yaml
# site.yaml
theme:
  font:
    family: "Inter, system-ui, sans-serif"
    file: "fonts/custom-font.woff2"
```

Body text uses the system UI font by default. `theme.font.family` replaces it
with a font stack of your own, written exactly as it goes in the stylesheet.
The first family in the list is the one the page asks for, so a font that is
already installed is enough: `family: "Atkinson Hyperlegible, system-ui,
sans-serif"` works with nothing else.

`file` names a font file in the config directory's `fonts/` folder, a
`woff2`, `woff`, `ttf` or `otf`:

```text
/config/site.yaml
/config/services.yaml
/config/fonts/custom-font.woff2
```

cairn serves it itself at `/fonts/`, so the page makes no external font
request: nothing to leak, nothing to block, no CDN. The first family in
`family` is declared as that font, which is how the two keys go together:
`file` supplies the file, `family` names it. Without `file`, the stack simply
uses whatever is installed.

One file is all cairn asks for, and a single-weight one is fine: the page uses
a few weights for names and headings, and the browser thickens the file it has
for the bold ones. A variable font covers them itself.

A font file that is not there is a `cairn -check` warning, not a broken page:
the browser falls back through the rest of the family list.

Headings keep the embedded display font, Fraunces; they are a `--font-display`
rule in [custom.css](#3-customcss), so leave that font alone and they stay.

## 3. custom.css

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
a visitor operates, the search box on the home page and the two buttons in the
leaving dialog, where nothing else says where to type or tap. That edge is held at
3:1 against the page and against its own fill, which is the floor
[WCAG asks for a control's boundary](https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html);
if you override it, keep it there.

`--border` sits near 2:1, and that number is a choice rather than a floor: 1.84
against the page and 2.04 against a card in light, 2.02 and 1.84 in dark. No
rule asks anything of it. It used to be a 1.2:1 hairline, which reads well on a
good screen in a dim room and vanishes on a cheap one in daylight, and a card
whose edge you cannot find is still a card you cannot scan. Take it wherever
suits your palette.

Anything beyond variables is regular CSS on stable class names (`.card`,
`.cat`, `.way`, `.tile`, `.about`, `.menu`, `.toc`, `.detail`, `.btn`, …).
Keep [WCAG AA
contrast](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
in mind for text; the defaults pass it.

### Typography

Headings are set in [Fraunces](https://fonts.google.com/specimen/Fraunces), a
variable serif embedded in the binary and served locally, no external font
request. Body text uses the system UI font, or the stack you set with
[`theme.font`](#2-typography). Override `--font-display` or `--font-body` in
`custom.css` to change either; `theme.font` sets `--font-body` and nothing
else.

## 4. Logo and favicon

```yaml
# site.yaml
logo: /assets/logo.png
```

Mount the file as described in [Icons: your own files](../recipes/icons.md#your-own-files).

### Artwork that cannot serve both themes

A logo drawn in one colour disappears in the theme it was not drawn for. It is
worth being precise about how badly: on cairn's icon tile, a black mark reads
**1.40:1** against the dark theme and a white one **1.24:1** against the light,
where the rules ask 3:1 of anything you are meant to see. A full-colour mark
usually clears both and needs nothing here.

Give the field two images instead of one, keyed by the theme each appears in:

```yaml
# site.yaml
logo:
  light: /assets/logo.svg # shown in the light theme
  dark: /assets/logo-white.svg # shown in the dark theme
favicon:
  light: /assets/fav.svg
  dark: /assets/fav-white.svg
```

```yaml
# services.yaml
- id: code
  url: https://git.example.org
  name: Code
  icon:
    light: github
    dark: github-light
```

Both keys are required. Half a pair paints nothing in one theme and leaves no
trace of why, so cairn refuses it at the file rather than at the first visitor
who switches. A plain string is still a plain string: one image, both themes,
and every config written before this existed goes on meaning what it meant.

The keys name the **theme**, not the ink. The icon collections do the opposite:
dashboard-icons ships `github-light.svg` as the pale mark for a dark
background, so it belongs under `dark:`. That reads backwards exactly once,
here, instead of every time you write it.

The logo and the icons follow the theme button, not just the system setting.
The favicon cannot: a browser tab is outside the page's stylesheet, so cairn
emits a second `<link>` with a `prefers-color-scheme` query, and the theme
button then overrides that query by hand. Support is a browser's business
rather than cairn's, and a browser that ignores it keeps the light one.

Two things have no theme to follow and always take the light image: the social
preview (`og:image`, which is a picture in someone else's chat window) and the
web app manifest (`icons`, which is a home-screen icon).

One cost, measured rather than guessed: a visitor whose system is light fetches
the light file alone, and the dark one only if they press the button. A visitor
whose system is dark fetches both, since the markup carries the light one and
the rule then asks for the other. It buys the artwork keeping its `alt`, its
dimensions and its lazy loading, and it is one small file.

Next: [Languages](i18n.md)
