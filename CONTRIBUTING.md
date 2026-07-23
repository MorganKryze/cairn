# Contributing

## Dev loop

```sh
go run ./src -config example    # http://localhost:8080, live-reloads example/
go test ./...
docker compose -f docker/compose.yaml up --build   # what CI and users get
```

The Docker build runs `go vet` and the test suite; if the image builds, it
passed.

## Layout

```text
src/        one Go package: config loading, rendering, HTTP; templates/ and
            assets/ are embedded into the binary
docker/     Dockerfile (FROM scratch) and the hardened reference compose
example/    the config served by the dev loop and the docs
docs/       GitHub-native Markdown, no generator
```

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

## Commits

One imperative line, scoped, short:

```text
config: merge multiple service files
docs: quickstart readme and example config
```

No trailers, no bodies unless the *why* genuinely needs a sentence.
