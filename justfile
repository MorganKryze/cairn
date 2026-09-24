# cairn dev tasks — https://just.systems

set quiet

[private]
default:
    just --list

# build the production image
build:
    docker build -f docker/Dockerfile -t cairn:local .

# lint the Helm chart and render the shapes CI renders
chart:
    helm lint charts/cairn
    helm template cairn charts/cairn >/dev/null
    helm template cairn charts/cairn --set ingress.enabled=true --set ingress.host=cairn.example.org --set ingress.tls.enabled=true >/dev/null
    helm template cairn charts/cairn --set 'imagePullSecrets[0].name=harbor' >/dev/null
    grep -qF 'kubeVersion: ">=1.19.0-0"' charts/cairn/Chart.yaml || { echo "kubeVersion is the Kubernetes floor, not cairn's version: it does not move with a release"; exit 1; }
    echo "chart ok"

# run vet + tests
test:
    go vet ./... && go test ./...

# measure coverage (add `-html=/tmp/cov.out` to the last line for a report)
# -coverpkg counts a package's code however it is reached: without it, each
# package only credits its own tests and the figure reads far too low.
coverage:
    go test -covermode=atomic -coverpkg=./src/... -coverprofile=/tmp/cov.out ./src/... >/dev/null
    go tool cover -func=/tmp/cov.out | tail -1

# run the linter and the format check
lint:
    golangci-lint run
    golangci-lint fmt --diff

# install the git pre-commit hook
hooks:
    git config core.hooksPath githooks
    echo "pre-commit hook active (bypass: git commit --no-verify)"

# start the demo stack (cairn + gatus + sample services)
demo:
    docker compose -f demo/compose.yaml up -d
    echo "cairn → http://localhost:8080   gatus → http://localhost:8081"

# rebuild the image and recreate the demo
demo-rebuild:
    docker compose -f demo/compose.yaml up -d --build --remove-orphans

# save a release to dist/ for an air-gapped move: just save 1.23.0 [linux/amd64]
save version platform="":
    #!/usr/bin/env bash
    set -euo pipefail
    # Checked before anything is fetched. A missing cosign fails the verify
    # below exactly the way a bad signature does, and it used to fail there,
    # after the pulls: the recipe printed "nothing here is trustworthy" about a
    # release that was fine, and left unverified files in dist/, the directory
    # that crosses the gap.
    command -v cosign >/dev/null || { echo "cosign is not installed, and without it this recipe cannot check a signature: https://docs.sigstore.dev/cosign/system_config/installation/" >&2; exit 1; }
    # docker save exports the platform you pulled and nothing else, so this is
    # an argument rather than an assumption: preparing an amd64 cluster from
    # an arm64 laptop is the classic way to learn that at CrashLoopBackOff
    # instead of here. Left out, it is this machine's, spelled the way the
    # rest of the project spells it: uname says aarch64, images say arm64.
    plat="{{ platform }}"
    if [ -z "$plat" ]; then
      case "$(uname -m)" in
        arm64|aarch64) plat=linux/arm64 ;;
        x86_64|amd64) plat=linux/amd64 ;;
        *) plat="linux/$(uname -m)" ;;
      esac
    fi
    mkdir -p dist
    docker pull --platform "$plat" morgankryze/cairn:{{ version }}
    # --platform on the save too, and it is load-bearing rather than belt and
    # braces. Under the containerd image store a tag keeps its whole index, so
    # saving by name exports every platform present and `docker load` then
    # materialises the host's: asking for amd64 on an arm64 laptop produced a
    # tar that loaded as arm64, silently, which is the CrashLoopBackOff this
    # recipe exists to prevent. Measured before it was written this way.
    docker save --platform "$plat" morgankryze/cairn:{{ version }} \
      -o "dist/cairn-{{ version }}-${plat//\//-}.tar"
    # The packaged chart rather than charts/ from git: it is the artifact the
    # release signed, and the only one whose version is not whatever the
    # working tree happens to say.
    helm pull oci://ghcr.io/morgankryze/charts/cairn --version {{ version }} --destination dist
    # Verified here on purpose, and this is the whole reason the recipe exists
    # rather than two docker commands: cosign queries the public transparency
    # log, so it is not something you get to do on the other side of the gap.
    # Both write their summary to stderr, hence 2>&1 and not just >/dev/null.
    for ref in morgankryze/cairn ghcr.io/morgankryze/charts/cairn; do
      cosign verify "$ref:{{ version }}" \
        --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null 2>&1 \
        || { echo "cosign could not verify $ref:{{ version }}; nothing here is trustworthy" >&2; exit 1; }
    done
    ls -lh dist/ | tail -n +2 | awk '{print "  " $9 "  " $5}'
    echo "signatures verified for $plat. Moving it across: docs/deployment/airgap.md"

# compare every response against a released tag, before cutting a release:
# just sweep v1.22.0
sweep tag:
    #!/usr/bin/env bash
    set -uo pipefail
    # Both binaries carry the same stamped version and both read the tag's own
    # config tree, so neither the version nor a fixture edited since can be the
    # difference: what is measured is the binary. Against main rather than the
    # tag would compare the tree with itself.
    # A leftover server from an earlier sweep answers the readiness loop
    # instantly and the run then measures that build instead. It cost one
    # phantom difference before this check existed; test-browser guards its own
    # ports for the same reason.
    for port in $(seq 8190 8195) $(seq 8290 8295); do
      if curl -fsS -o /dev/null "http://127.0.0.1:$port/healthz" 2>/dev/null; then
        echo "something already answers on 127.0.0.1:$port; kill it or this sweep measures it" >&2
        exit 1
      fi
    done
    wt=$(mktemp -d)
    trap 'git worktree remove --force "$wt" 2>/dev/null || true; kill $(jobs -p) 2>/dev/null || true' EXIT
    git worktree prune
    git worktree add -q --detach "$wt" {{ tag }} || exit 1
    go build -C "$wt" -trimpath -buildvcs=false -ldflags="-s -w -X main.version=sweep" -o /tmp/cairn-sweep-old ./src/cmd/cairn || exit 1
    go build -trimpath -buildvcs=false -ldflags="-s -w -X main.version=sweep" -o /tmp/cairn-sweep-new ./src/cmd/cairn || exit 1
    # -buildvcs=false on both, or the guard below never speaks: go stamps the
    # commit and a dirty flag into the binary, so a working tree that is the tag
    # with one file edited still produces different bytes.
    if [ "$(shasum -a 256 </tmp/cairn-sweep-old)" = "$(shasum -a 256 </tmp/cairn-sweep-new)" ]; then
      echo "the two builds are the same bytes: {{ tag }} is what is checked out" >&2; exit 1
    fi
    # The two servers answer on two ports and cairn writes its own origin into
    # sitemap.xml and robots.txt, so the port is normalised. LC_ALL=C or the
    # filter dies on the binary assets, and a filter that fails on both sides
    # reports equality. Asset names carry a digest of their contents: a changed
    # script is expected to move its own URL, and that is the one difference
    # read rather than counted.
    blur() { LC_ALL=C sed -E "s/127\.0\.0\.1:[0-9]+/HOST/g; s/\.[0-9a-f]{8}\.(css|js)/.DIGEST.\1/g"; }
    configs=(example scripts/fixtures/many-categories scripts/fixtures/status scripts/fixtures/themed scripts/fixtures/leave scripts/fixtures/states)
    total=0; same=0; left=0; oneside=0
    for i in "${!configs[@]}"; do
      cfg="${configs[$i]}"; po=$((8190+i)); pn=$((8290+i))
      /tmp/cairn-sweep-old -config "$wt/$cfg" -addr "127.0.0.1:$po" >/dev/null 2>&1 & o=$!
      /tmp/cairn-sweep-new -config "$wt/$cfg" -addr "127.0.0.1:$pn" >/dev/null 2>&1 & n=$!
      for p in $po $pn; do for _ in $(seq 1 30); do curl -fsS -o /dev/null "http://127.0.0.1:$p/healthz" 2>/dev/null && break; sleep .3; done; done
      mapfile -t pages < <(curl -fsS "http://127.0.0.1:$pn/sitemap.xml" | grep -o '<loc>[^<]*</loc>' | sed "s#<loc>http://127.0.0.1:$pn##;s#</loc>##")
      mapfile -t assets < <(for u in "${pages[@]}"; do curl -fsS "http://127.0.0.1:$pn$u"; done | grep -ao '/static/[A-Za-z0-9._/-]*' | sort -u)
      for u in "${pages[@]}" "${assets[@]}" /healthz /readyz /robots.txt /sitemap.xml /manifest.webmanifest /favicon.ico /.well-known/security.txt /custom.css /; do
        total=$((total+1))
        ao=$(curl -fsS "http://127.0.0.1:$po$u" -o /tmp/sweep-old 2>/dev/null; echo $?)
        an=$(curl -fsS "http://127.0.0.1:$pn$u" -o /tmp/sweep-new 2>/dev/null; echo $?)
        if [ "$ao" != "$an" ]; then oneside=$((oneside+1)); echo "  only one side answers: $cfg $u ($ao vs $an)"; continue; fi
        if [ "$ao" != 0 ]; then same=$((same+1)); continue; fi
        if cmp -s /tmp/sweep-old /tmp/sweep-new; then same=$((same+1)); continue; fi
        a=$(blur </tmp/sweep-old); b=$(blur </tmp/sweep-new)
        if [ -z "$a" ] || [ -z "$b" ]; then echo "the filter produced nothing for $u" >&2; exit 1; fi
        if [ "$a" = "$b" ]; then same=$((same+1)); else
          left=$((left+1)); echo "=== $cfg $u"; diff <(echo "$a") <(echo "$b") | head -14
        fi
      done
      # The control, once: two pages that differ, through the same filter. A
      # silent one here would mean the zero below is a broken instrument.
      if [ "$i" = 0 ]; then
        c=$(diff <(curl -fsS "http://127.0.0.1:$pn/en/" | blur) <(curl -fsS "http://127.0.0.1:$pn/en/pad/" | blur) | wc -l)
        [ "$c" -gt 10 ] || { echo "the control found $c differing lines between two different pages" >&2; exit 1; }
        echo "  control: $c diff lines between two different pages"
      fi
      kill $o $n 2>/dev/null || true; wait $o $n 2>/dev/null || true
    done
    echo
    echo "compared $total responses against {{ tag }}: $same identical once asset digests are blurred, $left different, $oneside on one side only"
    [ "$left" = 0 ] || exit 1

# check what a release actually published, from the registries: just verify 1.23.0
verify version:
    #!/usr/bin/env bash
    set -uo pipefail
    # Read the registries rather than the workflow's log. A job can report
    # success and still leave a tag behind: 1.20.1 published without moving
    # stable and latest, and only a query said so.
    fail=0
    echo "=== one object under every tag ==="
    seen=""
    for ref in ghcr.io/morgankryze/cairn morgankryze/cairn; do
      for t in {{ version }} "$(echo {{ version }} | cut -d. -f1-2)" "$(echo {{ version }} | cut -d. -f1)" stable latest; do
        d=$(docker buildx imagetools inspect "$ref:$t" --format '{{{{.Manifest.Digest}}}}' 2>/dev/null || echo MISSING)
        printf '  %-32s %-8s %s\n' "$ref" "$t" "$d"
        [ "$d" = MISSING ] && fail=1
        case " $seen " in *" $d "*) ;; *) seen="$seen $d" ;; esac
      done
    done
    [ "$(echo $seen | wc -w)" -eq 1 ] || { echo "  not one digest:$seen"; fail=1; }
    # cosign from its own image: a missing binary and a bad signature exit the
    # same way, and one release was called unsigned because it was not installed.
    echo "=== signatures, with a wrong identity as the control ==="
    cosign="docker run --rm ghcr.io/sigstore/cosign/cosign:latest"
    for ref in "morgankryze/cairn:{{ version }}" "ghcr.io/morgankryze/cairn:{{ version }}" "ghcr.io/morgankryze/charts/cairn:{{ version }}"; do
      if $cosign verify "$ref" --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null 2>&1; then echo "  signed   $ref"; else echo "  UNSIGNED $ref"; fail=1; fi
      if $cosign verify "$ref" --certificate-identity-regexp '^https://github.com/NotMorganKryze/' \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null 2>&1; then
        echo "  the control passed for $ref: this check accepts anything"; fail=1
      fi
    done
    echo "=== chart ==="
    helm show chart oci://ghcr.io/morgankryze/charts/cairn --version {{ version }} 2>/dev/null \
      | grep -E '^(version|appVersion|kubeVersion):' || { echo "  no chart {{ version }} published"; fail=1; }
    echo "=== binaries, and the compiler that built them ==="
    tmp=$(mktemp -d)
    gh release download "v{{ version }}" -D "$tmp" -p 'cairn_*' -p 'checksums.txt' 2>/dev/null || { echo "  no assets on the release"; fail=1; }
    [ -f "$tmp/checksums.txt" ] && { ( cd "$tmp" && shasum -a 256 -c checksums.txt ) || fail=1; }
    # The tarballs are built inside the image's own Go, and this is where that
    # promise is read back off the artifact instead of trusted.
    goimg=$(awk '$1 == "FROM" && $NF == "build" {print $3}' docker/Dockerfile)
    want=$(docker run --rm "$goimg" go env GOVERSION)
    for f in "$tmp"/cairn_*linux_amd64.tar.gz; do
      tar -xzf "$f" -C "$tmp"
      got=$(docker run --rm -v "$tmp:/t:ro" "$goimg" go version /t/cairn | awk '{print $2}')
      printf '  tarball built by %s, the pinned image says %s\n' "$got" "$want"
      [ "$got" = "$want" ] || { echo "  they disagree"; fail=1; }
    done
    rm -rf "$tmp"
    echo "=== pull size, which README.md states ==="
    index=$(docker buildx imagetools inspect "morgankryze/cairn:{{ version }}" --raw 2>/dev/null)
    if [ -z "$index" ]; then
      echo "  no image to weigh"
    else
      for arch in amd64 arm64; do
        d=$(echo "$index" | python3 -c "import json,sys; i=json.load(sys.stdin); print(next((m['digest'] for m in i['manifests'] if m.get('platform',{}).get('architecture')=='$arch' and m['platform']['os']=='linux'), ''))")
        [ -n "$d" ] || { echo "  $arch not in the index"; continue; }
        docker buildx imagetools inspect "morgankryze/cairn@$d" --raw | python3 -c "import json,sys; m=json.load(sys.stdin); print('  $arch', round((sum(l['size'] for l in m['layers'])+m['config']['size'])/1e6,1), 'MB')"
      done
    fi
    echo
    if [ "$fail" = 0 ]; then echo "all verified"; else echo "something is wrong"; exit 1; fi

# stop everything
down:
    docker compose -f demo/compose.yaml down

# follow the demo logs
logs:
    docker compose -f demo/compose.yaml logs -f

# refresh the README hero from the running demo; do this before every release
# (playwright lands in a gitignored node_modules, nothing is committed).
# The loop waits for Gatus to have polled, the way the CI demo job does. A fixed
# delay used to be enough until it wasn't: it caught the page with every pill on
# "unknown", which writes both files, fails nothing, and only shows up if
# someone opens the png.
shots: demo-rebuild
    @for i in $(seq 1 24); do p=$(curl -fs http://127.0.0.1:8080/en/ || true); echo "$p" | grep -q status-up && echo "$p" | grep -q status-down && break; [ "$i" = 24 ] && { echo "gatus never settled: every pill would read unknown" >&2; exit 1; }; sleep 5; done
    @npm --silent install --no-save --no-package-lock playwright
    @npx --yes playwright install --only-shell chromium
    @node scripts/screenshots.mjs

# drive search and the accessibility behaviours in a real browser: what the Go
# tests cannot see, since they assert the markup that ships and not what
# happens once someone types, clicks or scrolls
test-browser:
    #!/usr/bin/env bash
    # One shell for the whole recipe, and a trap. just runs each line of an
    # ordinary recipe in a shell of its own, so the pid was written on one line
    # and killed on another: aborting in between left cairn bound to 8090 for
    # good. The leak was the smaller half. On the next run the leftover
    # answered the readiness loop instantly, and the browser then drove the
    # build from before the change under test, green. Two instances now, so
    # both halves of that apply to every port: a stale server on any one of
    # them is enough to make the whole run measure yesterday's build.
    set -euo pipefail
    for port in 8090 8091 8092 8093 8094 8095; do
        if curl -fsS -o /dev/null "http://127.0.0.1:$port/healthz" 2>/dev/null; then
            echo "test-browser: something already answers on 127.0.0.1:$port, probably" >&2
            echo "a leaked cairn. Kill it, or this run measures that one instead." >&2
            exit 1
        fi
    done
    npm --silent install --no-save --no-package-lock playwright
    npx --yes playwright install --only-shell chromium
    go build -o /tmp/cairn-browser ./src/cmd/cairn
    # 8090 is the example config; 8091 is the fixture with nine categories,
    # enough for the trail to overflow its row, and a header burger, neither of
    # which example/ has; 8092 is
    # the one with a status page, which example/ has no business carrying;
    # 8093 is the one whose logo and icons differ between the two themes;
    # 8094 is the one that guards its external links with a dialog;
    # 8095 is the one carrying a card per state.
    /tmp/cairn-browser -config example -addr 127.0.0.1:8090 &
    pids=$!
    /tmp/cairn-browser -config scripts/fixtures/many-categories -addr 127.0.0.1:8091 &
    pids="$pids $!"
    /tmp/cairn-browser -config scripts/fixtures/status -addr 127.0.0.1:8092 &
    pids="$pids $!"
    /tmp/cairn-browser -config scripts/fixtures/themed -addr 127.0.0.1:8093 &
    pids="$pids $!"
    /tmp/cairn-browser -config scripts/fixtures/leave -addr 127.0.0.1:8094 &
    pids="$pids $!"
    /tmp/cairn-browser -config scripts/fixtures/states -addr 127.0.0.1:8095 &
    pids="$pids $!"
    trap 'kill $pids 2>/dev/null || true' EXIT INT TERM
    for port in 8090 8091 8092 8093 8094 8095; do
        ready=
        for _ in $(seq 1 20); do
            if curl -fsS -o /dev/null "http://127.0.0.1:$port/healthz" 2>/dev/null; then ready=1; break; fi
            sleep 1
        done
        [ -n "$ready" ] || { echo "test-browser: cairn never came up on $port" >&2; exit 1; }
    done
    node scripts/search.mjs http://127.0.0.1:8090/en/ http://127.0.0.1:8091/en/
    node scripts/a11y.mjs http://127.0.0.1:8090/en/ http://127.0.0.1:8091/en/ http://127.0.0.1:8092/en/ http://127.0.0.1:8093/en/ http://127.0.0.1:8094/en/ http://127.0.0.1:8095/en/
    node scripts/status.mjs http://127.0.0.1:8092/en/

# regenerate every icon from the one drawing in the script; checks the
# maskable safe zone the manifest promises Android
icons:
    @npm --silent install --no-save --no-package-lock playwright
    @npx --yes playwright install --only-shell chromium
    @node scripts/icons.mjs
