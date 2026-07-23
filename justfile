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
