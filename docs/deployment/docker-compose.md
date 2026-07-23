# Docker Compose

The reference setup. cairn is built for the strictest container settings;
use them, they cost nothing here.

```yaml
services:
  cairn:
    image: ghcr.io/morgankryze/cairn:latest
    # user: "1000:1000"    # optional; the image defaults to 65534 (nobody)
    ports:
      - 8080:8080
    volumes:
      - ./config:/config:ro
      # - ./assets:/assets:ro   # only if you serve your own images
    read_only: true
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: "0.50"
          memory: 128M
    healthcheck:
      test: ["CMD", "/cairn", "-healthcheck"]
      interval: 30s
      start_period: 5s
```

Why each line holds:

- **`:ro` mounts**: cairn only ever reads your files; edits still apply live
  because it polls the directory every two seconds.
- **`read_only: true`**: the container writes nothing, not even temp files.
- **`cap_drop: ALL`**: no capabilities needed: the image runs as `nobody`
  (65534) on port 8080.
- **`healthcheck`**: the image is `FROM scratch` (no shell, no curl), so the
  binary probes itself: `/cairn -healthcheck` hits `/healthz` and exits 0
  or 1.
- **`deploy.resources.limits`**: cairn idles at a few MB of RAM and near-zero
  CPU, so these ceilings are generous protection, not a requirement; raise or
  drop them freely.
- **`user`**: the binary needs no particular identity; uncomment to run it as
  any UID:GID (it defaults to `nobody`, 65534). Just make sure that user can
  read your mounted `/config`.
- **no network egress needed**: cairn makes zero outbound requests, except
  to a [`status.gatus` URL](../recipes/gatus.md) if you configure one. An
  internal-only egress policy is fine; note that icon *slugs* load in the
  visitor's browser from jsdelivr, not from the container
  ([avoid that if you want](../recipes/icons.md#your-own-files)).

## Updating

```sh
docker compose pull && docker compose up -d
```

Images are published to `ghcr.io/morgankryze/cairn`, always gated by the
test suite inside the build:

| Tag                   | Follows                                        |
| --------------------- | ---------------------------------------------- |
| `latest`, `stable`    | the newest release; what you want in production |
| `1`, `1.0`, `1.0.0`   | semver, from the release tags; pin as tight as you like |
| `unstable`            | every commit on `main`                         |
| a commit hash         | that exact build                               |

To build from source instead, replace `image:` with:

```yaml
    build:
      context: https://github.com/MorganKryze/cairn.git
      dockerfile: docker/Dockerfile
```

Next: [Podman](podman.md)
