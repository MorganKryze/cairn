# Changelog

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
