#!/usr/bin/env bash
# Feeds file lists through changes.sh and checks the flags it prints, so a
# pattern that stops matching fails here instead of skipping a check in CI.
set -uo pipefail
cd "$(dirname "$0")"
fail=0

# expect NAME "go web image chart ci" PATH...
expect() {
  local name=$1 want=$2
  shift 2
  local out got
  out=$(mktemp)
  printf '%s\n' "$@" | GITHUB_OUTPUT=$out bash changes.sh - >/dev/null
  got=$(for f in go web image chart ci; do sed -n "s/^$f=//p" "$out"; done | tr '\n' ' ')
  got=${got% }
  rm -f "$out"
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: want '$want', got '$got'"
    fail=1
  else
    echo "ok   $name"
  fi
}

expect docs-only "false false false false false" docs/index.md
expect readme-only "false false false false false" README.md
expect go "true false true false false" src/internal/config/config.go
expect template "true true true false false" src/internal/render/templates/layout.tmpl
expect testdata "true false true false false" src/internal/render/testdata/golden/gatus.html
expect chart "false false false true false" charts/cairn/values.yaml
expect workflow "true true true true true" .github/workflows/build.yml
expect classifier "true true true true true" .github/changes.sh
# --no-renames lists both sides of a move; the side left behind under src/ counts.
expect renamed-out-of-src "true false true false false" src/internal/old.go docs/old.go
expect accent "true false true false false" "src/internal/render/testdata/café.yml"

exit $fail
