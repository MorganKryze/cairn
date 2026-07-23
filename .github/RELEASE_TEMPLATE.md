<!-- Copy this into the release notes (gh release create vX.Y.Z --notes-file …).
     Sections in this order; delete a section entirely when it is empty. -->

One or two sentences: what this release is about, in plain words.

## ⚠️ Breaking

- What breaks, who is affected, and the exact migration step. Never leave
  it implied; delete the section only when there is truly nothing.

## ✨ Features

- New capability, phrased from the user's side: the config key, the page,
  the behavior they will notice.

## 🐛 Fixes

- What was wrong, now right.

## 🧹 Internal

- Chores, CI, docs, dependencies: anything invisible from the page.

## 📦 Image

```sh
docker pull ghcr.io/morgankryze/cairn:X.Y.Z
```

This release also moves `X.Y`, `X` (majors ≥ 1), `stable` and `latest`.
