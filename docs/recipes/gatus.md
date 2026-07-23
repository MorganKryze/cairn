# Pairing with a status page

cairn tells visitors what exists; a status page tells them what's up. They
pair well and stay separate — cairn never probes your services and never
asks the visitor's browser to.

Run [Gatus](https://github.com/TwiN/gatus) (or any status page) and link it
from the footer:

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

A Gatus endpoint per cairn service is usually enough:

```yaml
# gatus config.yaml
endpoints:
  - name: PDF toolbox
    url: https://pdf.example.org
    interval: 5m
    conditions: ["[STATUS] == 200"]
```

On the roadmap (v0.3): a `cairn`-to-Gatus config emitter, and optional status
dots on the cards fed by a Gatus API URL — server-side, still nothing probed
from the visitor's browser.

Next: [Reference](../reference.md)
