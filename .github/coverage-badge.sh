#!/usr/bin/env bash
# Regenerates the coverage badge SVG and publishes it to the orphan `badges`
# branch, from which the README serves it. Fully self-hosted: no third-party
# badge service, nothing about the repo leaves the repo.
set -euo pipefail

pct="${1:?usage: coverage-badge.sh PCT}"
color=$(awk -v p="$pct" 'BEGIN {
  if (p + 0 >= 80) print "#4c1";
  else if (p + 0 >= 70) print "#97ca00";
  else if (p + 0 >= 50) print "#dfb317";
  else print "#e05d44";
}')

cat > /tmp/coverage.svg <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="108" height="20" role="img" aria-label="coverage: ${pct}%">
  <title>coverage: ${pct}%</title>
  <clipPath id="r"><rect width="108" height="20" rx="3"/></clipPath>
  <g clip-path="url(#r)" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11">
    <rect width="62" height="20" fill="#555"/>
    <rect x="62" width="46" height="20" fill="${color}"/>
    <text x="31" y="14" fill="#fff" text-anchor="middle">coverage</text>
    <text x="85" y="14" fill="#fff" text-anchor="middle">${pct}%</text>
  </g>
</svg>
SVG

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
git add coverage.svg

if git diff --cached --quiet; then
  echo "badge unchanged (${pct}%)"
  exit 0
fi
git commit -m "coverage: ${pct}%"
git push origin badges
