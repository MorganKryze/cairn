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
- Two more pills exist and Gatus never lights them, because a Gatus result
  carries a pass/fail and nothing else. **Degraded** (amber) means it works
  but not well, and **Maintenance** (blue, and square rather than round)
  means off on purpose. They are there for the monitors that report more than
  a bool; each is a localized label in all seven languages, and both dots are
  measured against the pill they sit in on both themes.
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
"Unknown" and the log names the `x509` failure. Give cairn the authority that
signed it:

```yaml
status:
  gatus: https://status.internal
  ca: /assets/ca.crt                 # a file in the mounted assets directory
  # ca: https://pki.internal/ca.crt  # or an address cairn fetches it from
```

The bundle is added to the roots cairn already trusts, not swapped for them, so
a Gatus with a publicly signed certificate keeps working and adding a bundle
narrows nothing else. It is read once and cached, so a rotated authority needs
a restart, the same as a mounted one.

An `http://` URL is allowed, because an internal PKI often has nowhere better to
serve from and a bundle cannot be fetched over a certificate nobody trusts yet.
It is not free: over http, whatever sits on the path to that address decides
what cairn trusts for the poll, which is where `status.insecure` lands too.
cairn says which of the two it is at startup and on every `-check`.

There is also `status.insecure: true`, which turns the check off rather than
satisfying it. Both sides of the connection, and the symmetric fix for the
certificates Gatus itself has to trust, are in
[Air-gapped](../deployment/airgap.md#6-self-signed-certificates).

## 3. Keep the history across restarts

Gatus stores results in memory by default, and loses them when it restarts.
That shows up on cairn: with nothing stored, the API answers with endpoints and
no results, so every pill reads "Unknown" until the first check of each endpoint
lands. A file fixes it:

```yaml
storage:
  type: sqlite
  path: /data/data.db   # on a volume, or the file goes with the container
```

`postgres` is the other option, with a connection URL in the same `path`. Either
one survives a restart, so a Gatus that comes back up hands cairn the state it
already knew and the pills never blink.

## 4. Hiding what Gatus probes

If you point Gatus at an address you would rather not publish, an internal IP
or a URL carrying a token, its dashboard shows it by default. Three keys per
endpoint take it back:

```yaml
- name: alpha
  url: https://10.42.0.7:8443/
  conditions: ["[STATUS] == 200"]
  ui: { hide-url: true, hide-hostname: true, hide-port: true }
```

All three, because they hide three different things. What the API returns for
the same endpoint, with and without:

|                | plain                                                                          | hidden                                                                          |
| -------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- |
| `hostname`     | `"127.0.0.1"`                                                                  | absent                                                                          |
| a failed check | `Get "http://127.0.0.1:9/": dial tcp 127.0.0.1:9: connect: connection refused` | `Get "<redacted>": dial tcp <redacted>:<redacted>: connect: connection refused` |

`hide-url` redacts the address inside the error text as well, which is the leak
worth knowing about: it only appears when a check fails, so it is the one that
survives a test on a healthy endpoint. The port outlives the other two keys,
hence `hide-port`. Gatus also has `hide-errors`, which drops the message
entirely rather than redacting it.

None of this reaches cairn: the pills come from `name`, `key` and whether the
last check passed, and no `ui` setting hides any of those. `cairn -emit-gatus
-hide-targets` writes the block on every endpoint, so regenerating the file does
not lose it.

## 5. Link the status page

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

Next: [Coming from Homer or Homepage](migration.md)
