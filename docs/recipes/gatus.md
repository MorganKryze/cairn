# Status page (Gatus)

cairn tells visitors what exists; [Gatus](https://github.com/TwiN/gatus)
tells them what's up. cairn integrates both ways, and never probes your
services itself, nor asks the visitor's browser to.

## 1. Generate the Gatus config

The binary emits Gatus endpoints from your services: one endpoint per
service with an `http(s)` url (path-only urls are skipped), named after its
id, grouped by category:

```sh
docker run --rm -v ./config:/config:ro <your-cairn-image> -emit-gatus > gatus/endpoints.yaml
```

```yaml
endpoints:
  - name: pdf
    group: documents
    url: https://pdf.example.org
    interval: 5m          # how often Gatus probes the service
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

If cairn reaches Gatus over an internal network, for example
`http://gatus:8080` between containers, that address will not resolve in your
visitors' browsers. Set `status.page` to the public Gatus URL the pill should
link to:

```yaml
status:
  gatus: http://gatus:8080           # internal: cairn polls this, server-side
  page: https://status.example.org   # public: the pill links visitors here
```

When `status.page` is omitted, the pill links to `status.gatus`.

Each card gets a small status pill in its bottom-right corner (top of the
page on detail pages), a dot plus a localized label ("Online" /
"Offline"), and the pill links straight to that endpoint's page on your
Gatus (`…/endpoints/{group}_{name}`). The dot matches on endpoint
**name == service id**; the pill's link also needs **group == category**.
`-emit-gatus` produces both.

How it behaves, by design:

- **Online** breathes: the dot pulses slowly like a beacon. **Offline** is
  static and outlined so it stands out. Both cues work without relying on
  color alone, and the pulse stops under `prefers-reduced-motion`.
- The **server** polls `{gatus}/api/v1/endpoints/statuses`; visitors' browsers
  talk only to cairn (and to your Gatus, if they click the pill).
- Until Gatus has answered once (at boot, or while it is unreachable) every
  pill reads "Unknown" (neutral) rather than stale or absent, and the log
  says why.
- Once Gatus answers, a service it does not monitor simply shows no pill.

## 3. Link the status page

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

Next: [Reference](../reference.md)
