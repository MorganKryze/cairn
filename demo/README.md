# cairn demo

A complete, safe, throwaway playground: cairn, a real
[Gatus](https://github.com/TwiN/gatus), four real HTTP services and one
intentionally dead one. Prefer not to run anything? cairn lives publicly at
<https://cairn.libresoftware.cloud>.

```sh
docker compose up -d --build    # from this folder, or `just demo` from the repo root
```

Then open **<http://localhost:8080>**. The status dots start gray and turn
green or red within ~20 seconds, as the bundled Gatus reports in.

## What to try

- **The welcome note**: dismiss it with its corner button; a cookie keeps it
  closed for a year (delete the `about` cookie to bring it back).
- **Search from the keyboard**: start typing anywhere, or press ⌘K / Ctrl-K.
- **Languages**: `/` picks your browser's language; the header switcher
  remembers your choice.
- **Status pills**: green on the four live services, red on "Ghost service",
  each linking to its own endpoint page on the bundled Gatus (dashboard at
  <http://localhost:8081>), polled server-side every 10 s.
- **Detail pages**: "Who am I?", "Podinfo" and "Ghost service" have a
  "Learn more" link; the first two show preview images served from
  `config/media/`.
- **Search**: type `apa`, `réqu`, `echo`… accent-insensitive, no JS needed
  for the rest of the page.
- **Live reload**: edit `config/services.yaml` (rename something, add a
  card) and refresh: changes apply within ~2 s, no restart. Break the YAML
  on purpose: the site keeps serving and `docker compose logs cairn` tells
  you the file, the line and the expected shape.
- **Kill a service**: `docker compose stop welcome`, wait ~20 s, its dot
  turns red; `docker compose start welcome` brings it back.

## Network (isolated by design)

The whole stack lives on an `internal` Docker network with no route to the
outside: the application containers cannot reach the internet at all. The
only dual-homed piece is a dumb TCP gateway (`gateway/nginx.conf`) that
forwards the published ports; it holds no logic and no state. That is the
air-gap story cairn is built for, demonstrated: try
`docker exec cairn-demo-cairn-1 wget example.org` from any service, it has
nowhere to go.

The _page_ is air-gapped too, not just the containers: the service icons are
served from `assets/icons/` rather than the jsdelivr CDN, so loading this demo
in a browser makes **zero third-party requests**. That is the setup
[`cairn -emit-icons`](../docs/recipes/icons.md#going-fully-self-hosted)
produces for any site; refresh them with
`cairn -config demo/config -emit-icons`.

## Ports (all bound to 127.0.0.1; nothing is exposed to your network)

| Port | What                |
| ---- | ------------------- |
| 8080 | cairn               |
| 8081 | Gatus dashboard     |
| 8082 | whoami              |
| 8083 | nginx               |
| 8084 | Apache              |
| 8085 | podinfo             |
| 8086 | nothing (the ghost) |

One difference with a real deployment: here Gatus probes the services by
container name (`http://welcome:80`) while cairn's cards use the published
ports; with public HTTPS URLs both sides are identical and
[`cairn -emit-gatus`](../docs/recipes/gatus.md) writes the Gatus config for
you. Endpoint names match cairn service ids: that's the whole wiring.

## Clean up

```sh
docker compose down --rmi local
```
