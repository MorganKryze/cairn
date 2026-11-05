#!/usr/bin/env bash
# Says which parts of the repository a pull request touches, as flags in
# $GITHUB_OUTPUT that the jobs of build.yml, security.yml and codeql.yml read
# to decide whether they have anything to check.
#
# The workflows cannot use a `paths:` filter on pull_request for this: their
# jobs are required checks, and a workflow that never starts never reports
# them, which leaves a docs-only pull request blocked forever. A job skipped
# by its own `if:` does report, as skipped, and that satisfies the ruleset.
#
#   changes.sh BASE HEAD   flags for git diff BASE...HEAD
#   changes.sh -           flags for a file list read on stdin
#   changes.sh             every flag true: not a pull request
#
# Every way this can go wrong ends with every flag true. A skipped required
# check reads as a pass, so a classifier that fails quiet would let a pull
# request merge with nothing checked.
set -uo pipefail

flags="go web image chart ci"

all() {
  for f in $flags; do echo "$f=true"; done >>"$GITHUB_OUTPUT"
  echo "every check runs: $1"
  exit 0
}

case $# in
  0) all "not a pull request" ;;
  1) files=$(cat) ;;
  # --no-renames: rename detection prints only where a file went, so a Go
  # file moved to docs/ would read as a docs change and skip every check.
  # -z: without it, a path with a non-ASCII character or a quote comes out
  # wrapped in quotes and matches no pattern. A newline in a name splits it
  # in two, and the first piece still starts with the directory it is in.
  *) files=$(git diff --no-renames --name-only -z "$1...$2" | tr '\0' '\n') ||
    all "git diff $1...$2 failed" ;;
esac

go=false web=false image=false chart=false ci=false
while IFS= read -r f; do
  [ -n "$f" ] || continue
  # Everything under src/ is go, not only the .go files: templates and assets
  # are compiled in with go:embed, and go test holds the CSP hashes to
  # layout.tmpl and the rendered page to testdata/golden. In a case pattern
  # * crosses slashes, so src/* is every file under src.
  case $f in
    src/* | go.mod | go.sum | .golangci.yml) go=true ;;
  esac
  case $f in
    src/internal/render/templates/* | src/internal/render/assets/* | scripts/* | example/*) web=true ;;
  esac
  # README.md because the image job holds the README's stated pull size to
  # the image it builds.
  case $f in
    src/* | go.mod | go.sum | docker/* | demo/* | README.md) image=true ;;
  esac
  case $f in
    charts/*) chart=true ;;
  esac
  # A workflow, or this file, decides what runs: changing either runs it all.
  case $f in
    .github/workflows/* | .github/changes.sh) ci=true ;;
  esac
done <<<"$files"

if [ "$ci" = true ]; then
  all "a workflow changed"
fi

printf '%s\n' "$files"
{
  echo "go=$go"
  echo "web=$web"
  echo "image=$image"
  echo "chart=$chart"
  echo "ci=$ci"
} | tee -a "$GITHUB_OUTPUT"
