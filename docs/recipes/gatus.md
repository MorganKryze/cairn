# Status page (Gatus)

cairn tells visitors what exists; [Gatus](https://github.com/TwiN/gatus)
tells them what's up. cairn integrates both ways — and never probes your
services itself, nor asks the visitor's browser to.

## 1. Generate the Gatus config

The binary emits Gatus endpoints from your services — one endpoint per
service, named after its id, grouped by category:

```sh
docker run --rm -v ./config:/config:ro <your-cairn-image> -emit-gatus > gatus/endpoints.yaml
```

```yaml
endpoints:
  - name: pdf
    group: documents
    url: https://pdf.example.org
    interval: 5m
    conditions:
      - '[STATUS] == 200'
```

Merge it into your Gatus config (alerting stays yours). Adjust conditions
per service if 200 isn't the right health signal.

## 2. Feed the dots

Point cairn at the Gatus API:

```yaml
# site.yaml
status:
  gatus: https://status.example.org   # base URL of your Gatus instance
  interval: 60s                       # optional; default 60s, minimum 5s
```

Cards (and detail pages) get a small green or red dot with an accessible
localized label. The matching rule is the endpoint **name == service id** —
exactly what `-emit-gatus` produces.

How it behaves, by design:

- The **server** polls `{gatus}/api/v1/endpoints/statuses`; visitors' browsers
  talk only to cairn.
- A service with no matching endpoint simply shows no dot.
- If Gatus stops answering, the dots disappear rather than go stale, and the
  log says why.

## 3. Link the status page

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

Next: [Reference](../reference.md)
