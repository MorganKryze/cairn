#!/usr/bin/env bash
# Regenerates the badge SVGs and publishes them to the orphan `badges` branch,
# from which the README serves them. Fully self-hosted: no third-party badge
# service, nothing about the repo leaves the repo.
#
# Two numbers rather than one, because either alone flatters. A coverage
# percentage says nothing about how much was written; a count of tests says
# nothing about what they reach.
set -euo pipefail

pct="${1:?usage: badges.sh PCT TESTS}"
tests="${2:?usage: badges.sh PCT TESTS}"

color=$(awk -v p="$pct" 'BEGIN {
  if (p + 0 >= 80) print "#4c1";
  else if (p + 0 >= 70) print "#97ca00";
  else if (p + 0 >= 50) print "#dfb317";
  else print "#e05d44";
}')

# badge LABEL VALUE LABEL_WIDTH VALUE_WIDTH COLOUR NAME
#
# The widths are passed in rather than worked out here: this writes the SVG by
# hand instead of pulling in a renderer, and a shell script has no font
# metrics. They were measured elsewhere, in Verdana at 11px, the first family
# the SVG asks for, and they are what makes the two badges look like a pair:
# roughly 6px of air on each side of a label, 4.5px on each side of a value.
badge() {
  local w=$(($3 + $4))
  cat >"/tmp/$6.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="20" role="img" aria-label="$1: $2">
  <title>$1: $2</title>
  <clipPath id="r"><rect width="${w}" height="20" rx="3"/></clipPath>
  <g clip-path="url(#r)" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11">
    <rect width="$3" height="20" fill="#555"/>
    <rect x="$3" width="$4" height="20" fill="$5"/>
    <text x="$(($3 / 2))" y="14" fill="#fff" text-anchor="middle">$1</text>
    <text x="$(($3 + $4 / 2))" y="14" fill="#fff" text-anchor="middle">$2</text>
  </g>
</svg>
SVG
}

badge coverage "${pct}%" 62 46 "$color" coverage

# A count is the one value here that can gain a character, so its box is the
# one that has to follow: a digit is exactly 7px wide in Verdana at 11px, and
# 9px is the air the coverage value already carries. A box wide enough for a
# percentage is what left `505` floating in the middle of its own.
#
# Even widths on purpose where the choice exists: the text is centred on
# width/2 in shell arithmetic, which truncates, so an odd box sits its text
# half a pixel off centre.
badge tests "$tests" 38 "$((9 + 7 * ${#tests}))" "#4c1" tests

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

if git ls-remote --exit-code --heads origin badges >/dev/null 2>&1; then
  git fetch --depth 1 origin badges:badges
  git switch badges
else
  git switch --orphan badges
fi

git rm -rqf . >/dev/null 2>&1 || true
cp /tmp/coverage.svg coverage.svg
cp /tmp/tests.svg tests.svg
git add coverage.svg tests.svg

if git diff --cached --quiet; then
  echo "badges unchanged (${pct}%, ${tests} tests)"
  exit 0
fi
git commit -m "coverage ${pct}%, ${tests} tests"
git push origin badges
