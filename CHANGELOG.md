# Changelog

## 0.6.0 - 2026-07-23

- Header: a second row with your navigation links (built-in inline glyph
  icons, or your own images) and a compact search that grows on focus and
  sleeps, grayed, on pages it cannot filter.
- `about`: a dismissable welcome note under the header; one-year cookie,
  hidden before first paint so it never flashes.
- `pages`: cairn serves legal notice / privacy pages itself, with titled
  `sections`, auto-linked at the end of the footer.
- A category trail in the right margin follows your scroll and jumps on
  click; entries mirror the search filter.
- Search reachable from anywhere: type-to-search and ⌘K / Ctrl-K.
- A header toggle forces light or dark; the system preference still rules
  until the visitor picks, and the choice stays in their browser.
- Cards: equal heights within a row, "Learn more" flows after the
  description, the status pill moves to the bottom-right corner and links to
  its own Gatus endpoint page.
- Language: only the switcher pins the choice; until then the browser
  language is followed on every visit.
- Footer pinned to the viewport floor on short pages.
- CI publishes ghcr.io/morgankryze/cairn: `unstable` + commit hash on every
  code change to main, semver + `stable` + `latest` on releases; a justfile
  wraps the everyday commands.

## 0.5.1 - 2026-07-23

- Per-card status moved to a labelled pill in the card's top-right corner
  ("Online" / "Offline"), and the pill links to the Gatus status page.
- Online pills breathe like a beacon (a slow, subtle pulse), disabled under
  `prefers-reduced-motion`; offline is static and outlined.

## 0.5.0 - 2026-07-23

- Visual identity pass: the cairn (a stack of hand-placed stones) is now the
  brand mark, echoed in the hero and as the neutral icon fallback.
- Headings set in Fraunces, a variable serif embedded in the binary and
  served locally, no external font request. Body stays on the system font.
- Category headers reworked as trail "waypoints" (marker, name, hairline,
  count); cards get consistent icon tiles, softer depth and a clearer hover.
- Cooler mineral palette, refined light/dark tokens, `--accent-ink` derived
  for AA-contrast accent text; status dots redrawn (gray/green/red, each
  distinct beyond color).
- Subtle hero reveal on load, disabled under `prefers-reduced-motion`.

## 0.4.0 - 2026-07-23

- `demo/`: a one-command playground: cairn, a real Gatus and five sample
  services (one intentionally down), everything bound to 127.0.0.1.
- Status dots start gray ("status unknown") until Gatus answers, then turn
  green or red; they also fall back to gray during a Gatus outage.

## 0.3.0 - 2026-07-23

- `cairn -emit-gatus` prints a Gatus endpoints config derived from the
  services (endpoint name = service id, group = category).
- Optional status dots on cards and detail pages, fed server-side from a
  Gatus API (`status.gatus` and `status.interval` in `site.yaml`), with
  localized accessible labels. Dots disappear instead of going stale when
  Gatus stops answering.

## 0.2.0 - 2026-07-23

- `categories.yaml`: localized category names and explicit ordering.
- Per-service detail pages at `/{locale}/{id}/`: a `details:` field renders
  paragraphs behind a "Learn more" link on the card.
- `custom.css` escape hatch, served from `/config` and loaded last.
- Service ids validated as URL-safe slugs.
- Full documentation tree under `docs/`.

## 0.1.0 - 2026-07-23

- Guest-first directory: hero, categories, cards, accent theming, light and
  dark, accent-insensitive fuzzy search as progressive enhancement.
- Structural i18n: translated fields inline, `Accept-Language` negotiation,
  remembered language switcher, per-locale meta and OpenGraph, built-in UI
  strings (fr, en) overridable from `site.yaml`.
- Boring ops: single ~12 MB `FROM scratch` image, non-root, read-only, YAML
  mounted at `/config` with ~2 s live reload, config errors with file, line
  and expected shape, `/healthz` plus a self-probe flag for scratch
  healthchecks.
