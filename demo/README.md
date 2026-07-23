# cairn demo

A complete, safe, throwaway playground: cairn, a real
[Gatus](https://github.com/TwiN/gatus), four real HTTP services and one
intentionally dead one.

```sh
docker compose up -d --build
```

Then open **<http://localhost:8080>**. The status dots start gray and turn
green or red within ~20 seconds, as the bundled Gatus reports in.

## What to try

- **Languages** — `/` picks your browser's language; the header switcher
  remembers your choice.
- **Status dots** — green on the four live services, red on "Ghost service".
  They are fed by the bundled Gatus (dashboard at
  <http://localhost:8081>), polled server-side every 10 s.
- **Detail pages** — "Who am I?" and "Ghost service" have a "Learn more"
  link.
- **Search** — type `apa`, `réqu`, `echo`… accent-insensitive, no JS needed
  for the rest of the page.
- **Live reload** — edit `config/services.yaml` (rename something, add a
  card) and refresh: changes apply within ~2 s, no restart. Break the YAML
  on purpose: the site keeps serving and `docker compose logs cairn` tells
  you the file, the line and the expected shape.
- **Kill a service** — `docker compose stop welcome`, wait ~20 s, its dot
  turns red; `docker compose start welcome` brings it back.

## Ports (all bound to 127.0.0.1 — nothing is exposed to your network)

| Port | What                    |
| ---- | ----------------------- |
| 8080 | cairn                   |
| 8081 | Gatus dashboard         |
| 8082 | whoami                  |
| 8083 | nginx                   |
| 8084 | Apache                  |
| 8085 | podinfo                 |
| 8086 | nothing — the ghost     |

One difference with a real deployment: here Gatus probes the services by
container name (`http://welcome:80`) while cairn's cards use the published
ports; with public HTTPS URLs both sides are identical and
[`cairn -emit-gatus`](../docs/recipes/gatus.md) writes the Gatus config for
you. Endpoint names match cairn service ids — that's the whole wiring.

## Clean up

```sh
docker compose down --rmi local
```
