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

# save a release to dist/ for an air-gapped move: just save 1.20.2 [linux/amd64]
save version platform="":
    #!/usr/bin/env bash
    set -euo pipefail
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
    node scripts/search.mjs http://127.0.0.1:8090/en/
    node scripts/a11y.mjs http://127.0.0.1:8090/en/ http://127.0.0.1:8091/en/ http://127.0.0.1:8092/en/ http://127.0.0.1:8093/en/ http://127.0.0.1:8094/en/ http://127.0.0.1:8095/en/
    node scripts/status.mjs http://127.0.0.1:8092/en/

# regenerate every icon from the one drawing in the script; checks the
# maskable safe zone the manifest promises Android
icons:
    @npm --silent install --no-save --no-package-lock playwright
    @npx --yes playwright install --only-shell chromium
    @node scripts/icons.mjs
