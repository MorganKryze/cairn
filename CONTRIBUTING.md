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
`test`, `test-search`, `lint`, `build`, `demo`, `demo-rebuild`, `down`,
`logs`, `hooks`.
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

## Releasing

`just shots` regenerates the README hero, light and dark, from the running
demo. Run it before every release: the screenshot is the first thing anyone
sees, and it goes stale quietly. Playwright installs itself into a gitignored
`node_modules/` on demand, so nothing is added to the repository.

## Conduct

Be decent, assume good faith, critique the work and not the person. The full
version is in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
