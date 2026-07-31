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

When `status.page` is omitted, the pill links to `status.gatus`. If you have
no Gatus your visitors should reach at all, say so instead:

```yaml
status:
  gatus: http://gatus:8080           # internal, and staying that way
  linked: false                      # the pills state the status, nothing more
```

`linked: false` keeps the dot and its label and drops the link. The pills
stop being controls: no target, no keyboard stop, and on a card a click goes
where the rest of the card goes, to the service itself. Use it when your
Gatus is internal, or when you would rather visitors saw the state than went
looking at a monitoring dashboard.

Each card gets a small status pill in its bottom-right corner (top of the
page on detail pages), a dot plus a localized label ("Online" /
"Offline"), and the pill links straight to that endpoint's page on your
Gatus. The dot matches on endpoint **name == service id**, which is the one
thing your Gatus config has to share with cairn; `-emit-gatus` writes it.

The link needs no agreement at all. Gatus reports the key of its own endpoint
page (`…/endpoints/{group}_{name}`) alongside each status, and cairn uses what
it reports, so your endpoints can be grouped however you like, or not grouped
at all. Until Gatus has answered, the pills fall back to the key `-emit-gatus`
would have produced.

How it behaves, by design:

- **Online** breathes: the dot pulses slowly like a beacon. **Offline** is
  static and outlined so it stands out. Both cues work without relying on
  color alone, and the pulse stops under `prefers-reduced-motion`.
- The **server** polls `{gatus}/api/v1/endpoints/statuses`; visitors' browsers
  talk only to cairn (and to your Gatus, if they click the pill).
- Until Gatus has answered once (at boot, or while it is unreachable) every
  pill reads "Unknown" (neutral) rather than stale or absent, and the log
  says why. cairn keeps asking on every interval; it says so once and stays
  quiet after that, rather than repeating itself until Gatus is back.
- Once Gatus answers, a service it does not monitor simply shows no pill.
- **An open page keeps up.** Start cairn before Gatus and every pill reads
  Unknown, as it should; when Gatus comes up, the tab already open catches up
  on its own, without a reload. It refetches the page it is on at
  `status.interval` and replaces the pills, nothing else: no separate API, and
  the request goes to cairn, never to Gatus. A tab in the background does not
  poll, and a pill your keyboard is on is left alone until you move off it.
  Pages of a site with no `status.gatus` do not ship the script at all.

If your Gatus answers over `https://` with a certificate cairn does not know,
which is the usual state of affairs on an internal network, every pill reads
"Unknown" and the log names the `x509` failure. The fix, along with the
symmetric one for the certificates Gatus itself has to trust, is in
[Air-gapped](../deployment/airgap.md#6-self-signed-certificates). There is also
`status.insecure: true`, which turns the check off rather than satisfying it;
the same page says what that costs.

## 3. Link the status page

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

Next: [Coming from Homer or Homepage](migration.md)
