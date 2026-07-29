# Contributing

## Dev loop

```sh
go run ./src/cmd/cairn -config example    # http://localhost:8080, live-reloads example/
go test ./...
docker compose -f docker/compose.yaml up --build   # what CI and users get
```

The Docker build runs `go vet` and the test suite; if the image builds, it
passed.

With [just](https://just.systems) installed, `just` lists the shortcuts:
`test`, `test-search`, `lint`, `build`, `chart`, `demo`, `demo-rebuild`,
`down`, `logs`, `hooks`, `icons`.
Linting is [golangci-lint](https://golangci-lint.run) with a near-default
config (`.golangci.yml`); CI runs it on every code push.

Run `just hooks` once: it points git at `githooks/`, whose pre-commit runs
gofmt, vet and the tests when Go files are staged, lints staged workflows,
and enforces a couple of house style rules on the way.

## Layout

```text
src/cmd/cairn/        wiring only: flags, startup, hand off to the server
src/internal/
  config/             read and validate the YAML; depends on nothing
  render/             config to bytes; templates/ and assets/ embed here
  status/             the Gatus client
  server/             routes, handlers, probes, the two background loops
  check/              backs -check: validates like a boot would
  testutil/           the one helper shared by every package's tests
docker/               Dockerfile (FROM scratch) and the hardened compose
charts/cairn/         the Helm chart, same objects as docs/deployment/kubernetes.md
example/              the config served by the dev loop and the docs
docs/                 GitHub-native Markdown, no generator
```

The dependency graph runs one way: `config` knows nothing, `render` and
`status` know `config`, `server` knows all three, `main` wires them. Keep it
that way. A test lives beside the package it exercises; the two that genuinely
span a boundary use an external `_test` package rather than pulling a
dependency backwards.

## Ground rules

- **Scope.** cairn is a directory, not a control panel: no auth, widgets,
  Docker integration, metrics, admin UI, database or per-user state. PRs
  adding those will be declined kindly, with a pointer to
  [docs/comparison.md](docs/comparison.md).
- **Config errors are product.** Anything a user can get wrong in YAML must
  fail with the file, the line, the problem and the expected shape. A test
  proves it.
- **Docs are product.** A key without documentation, or a docs page that
  doesn't copy-paste-run, is a bug. Update `docs/` in the same PR.
- **Dependencies.** The stdlib first; currently the only module dependency is
  `yaml.v3` and it should stay that way unless something is truly impossible
  without.
- **Accessibility and i18n are not optional.** New UI text goes through the
  strings table (both built-in locales), keyboard and contrast stay intact.

## Add your language

The friendliest first contribution: cairn's interface text lives in
`src/locales.go`, one small block per language, about eighteen short
strings. Copy the English block, translate it, and open a pull request; a
test checks completeness, so if `just test` passes, your table is done.
Regional variants only need their own block when they genuinely differ
(`pt-BR` already finds `pt`).

## Commits

One imperative line, scoped, short:

```text
config: merge multiple service files
docs: quickstart readme and example config
```

No trailers, no bodies unless the *why* genuinely needs a sentence.

## The search

`search.js` is the only behaviour `go test` cannot reach: the Go tests assert
the markup that ships, not what happens once someone types into it. `just
test-search` drives it in a real browser against the example config, and CI
runs the same script. It pulls nothing from a registry, so unlike the demo job
it is safe to make a required check.

Anything you change in `search.js` belongs there, particularly the paths back
to an empty query: clearing the field, Escape, and typing only spaces all have
to return the full list rather than "no results".

## The icons

Every icon comes from one drawing, in `scripts/icons.mjs`. `just icons`
regenerates the lot: the five files the binary embeds and serves, and the three
in `docs/assets/brand/` that the icon collections ask for, cropped to the mark
rather than to its padded square. Running it on an unchanged mark rewrites the
same bytes, so a diff after `just icons` means the drawing moved.

Change the mark there and nowhere else. The same shapes are inlined in
`layout.tmpl` as the icon a service without one falls back to, and
`TestTheMarkIsDrawnTheSameInBothPlaces` fails if that copy stops matching
`favicon.svg`; it has drifted a whole release before.

The script also checks what the manifest promises: the app icons are declared
`maskable`, which tells Android nothing lives outside the middle 80% of the
square. It measures the rendered ink and exits non-zero if a redrawn mark
breaks that.

## Releasing

`just shots` regenerates the README hero, light and dark, from the running
demo. Run it before every release: the screenshot is the first thing anyone
sees, and it goes stale quietly. Playwright installs itself into a gitignored
`node_modules/` on demand, so nothing is added to the repository.

The chart needs nothing at release time. CI packages it with the tag as both
`version` and `appVersion`, so chart 1.2.3 installs cairn 1.2.3 and the numbers
in `Chart.yaml` only matter to someone installing from a checkout. Changing the
chart means changing [docs/deployment/helm.md](docs/deployment/helm.md) in the
same PR: `values.yaml` is a public API, and renaming a key breaks installs.

## Conduct

Be decent, assume good faith, critique the work and not the person. The full
version is in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
