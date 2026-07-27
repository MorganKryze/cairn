# cairn dev tasks — https://just.systems

set quiet

[private]
default:
    just --list

# build the production image
build:
    docker build -f docker/Dockerfile -t cairn:local .

# run vet + tests
test:
    go vet ./... && go test ./...

# measure coverage (add `-html=/tmp/cov.out` to the last line for a report)
coverage:
    go test -covermode=atomic -coverprofile=/tmp/cov.out ./... >/dev/null
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
    docker compose -f demo/compose.yaml up -d --build

# stop everything
down:
    docker compose -f demo/compose.yaml down

# follow the demo logs
logs:
    docker compose -f demo/compose.yaml logs -f

# refresh the README hero from the running demo; do this before every release
# (playwright lands in a gitignored node_modules, nothing is committed)
shots: demo-rebuild
    @sleep 4
    @npm --silent install --no-save --no-package-lock playwright
    @npx --yes playwright install --only-shell chromium
    @node scripts/screenshots.mjs
