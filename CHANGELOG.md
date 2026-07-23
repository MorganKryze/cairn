# Changelog

## 0.5.0 — 2026-07-23

- Visual identity pass: the cairn (a stack of hand-placed stones) is now the
  brand mark, echoed in the hero and as the neutral icon fallback.
- Headings set in Fraunces, a variable serif embedded in the binary and
  served locally — no external font request. Body stays on the system font.
- Category headers reworked as trail "waypoints" (marker, name, hairline,
  count); cards get consistent icon tiles, softer depth and a clearer hover.
- Cooler mineral palette, refined light/dark tokens, `--accent-ink` derived
  for AA-contrast accent text; status dots redrawn (gray/green/red, each
  distinct beyond color).
- Subtle hero reveal on load, disabled under `prefers-reduced-motion`.

## 0.4.0 — 2026-07-23

- `demo/`: a one-command playground — cairn, a real Gatus and five sample
  services (one intentionally down), everything bound to 127.0.0.1.
- Status dots start gray ("status unknown") until Gatus answers, then turn
  green or red; they also fall back to gray during a Gatus outage.

## 0.3.0 — 2026-07-23

- `cairn -emit-gatus` prints a Gatus endpoints config derived from the
  services (endpoint name = service id, group = category).
- Optional status dots on cards and detail pages, fed server-side from a
  Gatus API (`status.gatus` and `status.interval` in `site.yaml`), with
  localized accessible labels. Dots disappear instead of going stale when
  Gatus stops answering.

## 0.2.0 — 2026-07-23

- `categories.yaml`: localized category names and explicit ordering.
- Per-service detail pages at `/{locale}/{id}/` — a `details:` field renders
  paragraphs behind a "Learn more" link on the card.
- `custom.css` escape hatch, served from `/config` and loaded last.
- Service ids validated as URL-safe slugs.
- Full documentation tree under `docs/`.

## 0.1.0 — 2026-07-23

- Guest-first directory: hero, categories, cards, accent theming, light and
  dark, accent-insensitive fuzzy search as progressive enhancement.
- Structural i18n: translated fields inline, `Accept-Language` negotiation,
  remembered language switcher, per-locale meta and OpenGraph, built-in UI
  strings (fr, en) overridable from `site.yaml`.
- Boring ops: single ~12 MB `FROM scratch` image, non-root, read-only, YAML
  mounted at `/config` with ~2 s live reload, config errors with file, line
  and expected shape, `/healthz` plus a self-probe flag for scratch
  healthchecks.
