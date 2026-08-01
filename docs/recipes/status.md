# Status page

cairn tells visitors what exists; a monitor tells them what is up. cairn reads
one, and never probes your services itself, nor asks the visitor's browser to.

Three shapes are read:

| monitor                                                | what cairn needs                                     | what it gets                                                      |
| ------------------------------------------------------ | ---------------------------------------------------- | ----------------------------------------------------------------- |
| [Gatus](https://github.com/TwiN/gatus)                 | `status.gatus`                                       | per-service pills, each linking to that service's own page        |
| [Uptime Kuma](https://github.com/louislam/uptime-kuma) | `status.url`, `status.provider: kuma`, `status.slug` | per-service pills, all linking to the published status page       |
| [any other status API](#any-other-status-api)          | `status.url`, `status.provider: json`, `status.map`  | the same, from any document shaped like a list of `{name, state}` |

Gatus first, because it integrates both ways: cairn can write its config.
[Uptime Kuma](#uptime-kuma) needs no agreement beyond naming each monitor after
the service id. The [mapper](#any-other-status-api) covers Statuspage,
Instatus, Upptime, Cachet, UptimeRobot, Better Stack and whatever is written
next; each of those was read from a live instance, and the mapping it took is
in the table.

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

## Uptime Kuma

Kuma publishes a **status page**, and that page answers two endpoints without
a login. cairn reads those:

```yaml
# site.yaml
status:
  provider: kuma
  url: https://kuma.example.org # the Kuma instance
  slug: tools # the published status page, from its own URL
  interval: 60s
```

The slug is the last part of the status page's URL:
`https://kuma.example.org/status/tools` is `tools`. Kuma serves statuses per
status page rather than per instance, so the slug is half the address, and
cairn refuses to start without it rather than polling an endpoint that cannot
exist.

Three things have to be true, and only the first is work:

1. **Each monitor is named after the cairn service id.** That is the one
   agreement, the same one Gatus needs. Kuma has no config file and no setup
   API, so there is no `cairn -emit-kuma` and there cannot be one: everything
   in Kuma is created by clicking, which is exactly how the instance this was
   tested against was set up, with a script driving a browser.
2. **The monitors are attached to the status page**, in a group. A page with
   none is a page that answers with no monitor, and cairn says so with the
   slug in the message.
3. **The status page is published.** An unpublished one 404s exactly as a
   misspelled slug does, which is why the error names both possibilities.

What differs from Gatus:

- **The pill links to the status page**, the same one for every service, since
  Kuma has no page per monitor. Without `status.page`, cairn derives it from
  the address and the slug, which is the URL Kuma itself serves it at.
- **Maintenance shows.** Kuma reports it, so a monitor under maintenance gets
  the blue square pill rather than a red dot. It reports no degraded, and cairn
  invents none.
- **Pending reads as Unknown.** A monitor waiting for its first check has not
  said anything yet, which is what the neutral pill already means.

## Any other status API

Most status pages answer with the same shape: a list of objects, each carrying
a name and a state. `status.provider: json` reads any of them, with a mapping
that says where to look.

```yaml
# site.yaml, reading an Atlassian Statuspage
status:
  provider: json
  url: https://www.githubstatus.com/api/v2/components.json
  map:
    list: components # path to the array
    key: name # field holding the service name
    state: status # field holding its state
    up: [operational]
    degraded: [degraded_performance, partial_outage]
    maintenance: [under_maintenance]
```

A path is a dotted walk and nothing more: `attributes.status` reaches inside a
nested object, and `list:` left out means the document is itself the array.
There are no wildcards and no filters, because anything cleverer is a query
language, and a query language in YAML is a second product.

The value lists are **allow-lists**: a state in none of them reads as down.
That is deliberate in both directions. A vendor that adds a word next year
cannot make a broken service look green, and an operator who forgot to list one
sees a red pill rather than a wrong one.

`unknown` is the exception, read before the rest. Put a monitor's paused state
there and the service gets no pill at all instead of a red one, which is what
cairn's neutral state has always meant.

### Mappings that were run against the real thing

Each row below was read by cairn itself on 2026-08-01, against that live
service, and the count is what came back. Component names are the one
agreement: a pill is drawn only where a name matches a cairn service id, so on
your own status page, name the components after your service ids.

| service                   | endpoint                                        | `list`         | `key`                    | `state`             | `up`                    | read                                           |
| ------------------------- | ----------------------------------------------- | -------------- | ------------------------ | ------------------- | ----------------------- | ---------------------------------------------- |
| Atlassian Statuspage      | `/api/v2/components.json`                       | `components`   | `name`                   | `status`            | `operational`           | 471 on Cloudflare, 33 on Discord, 12 on GitHub |
| Instatus                  | `/v2/components.json`                           | `components`   | `name`                   | `status`            | `OPERATIONAL`           | 1                                              |
| Upptime                   | `history/summary.json` in the repo              | (none)         | `slug`                   | `status`            | `up`                    | 4                                              |
| Better Stack, public page | `/index.json`                                   | `included`     | `attributes.public_name` | `attributes.status` | `operational`           | 4                                              |
| UptimeRobot, public page  | `stats.uptimerobot.com/api/getMonitorList/<id>` | `psp.monitors` | `name`                   | `statusClass`       | `success`               | 15, of which 5 paused                          |
| Cachet                    | `/api/v1/components?per_page=100`               | `data`         | `name`                   | `status_name`       | your instance's wording | 31 on status.framasoft.org                     |

Notes worth having before you write yours:

- **Statuspage's other states** are `degraded_performance` and `partial_outage`
  for degraded, `under_maintenance` for maintenance. Cloudflare's page had 34
  and 20 of them respectively while this was being written, so the two newer
  pills are not theoretical.
- **UptimeRobot** calls a switched-off monitor `paused`. Put it in `unknown`,
  or five paused monitors show as five outages.
- **Cachet** answers `status` as a number, which no mapping reads; use
  `status_name`, whose wording follows the language of that Cachet instance.
  It also paginates at 20, hence `?per_page=100`.
- **Better Stack and StatusCake** also have token APIs, which work the same way
  with `token_file`; the public status page needs no credential at all.

### The token, if the API needs one

Never in `site.yaml`. cairn takes the **path of a file** holding it:

```yaml
status:
  provider: json
  url: https://uptime.betterstack.com/api/v2/monitors
  token_file: /run/secrets/status-token # a mounted file, not a value
  token_scheme: Bearer # the default; Statuspage wants OAuth
  map: { list: data, key: attributes.pronounceable_name, state: attributes.status, up: [up] }
```

A Kubernetes Secret, a docker compose secret and a Vault agent all deliver
exactly that: a file. cairn reads it on every poll, so a rotated token takes
effect without a restart, and it travels in an `Authorization` header, never in
the URL. A poll error names the key rather than printing the credential.

The file may not live under `/assets`. That directory is served to every
visitor, so a token there would be published rather than stored, and cairn
refuses to start. `status.ca` under `/assets` stays legal, and the asymmetry is
the point: a CA certificate is the public half of an authority, and a token is
not the public half of anything.

### Two that cairn does not read, and why

**HetrixTools** puts the token in the URL path
(`/v1/<TOKEN>/uptime/monitors/…`). cairn logs a failed poll with the address it
called, so the token would end up in cairn's own log.

**Site24x7** needs an OAuth refresh-token exchange, which would make cairn hold
rotating state. That is an architectural line rather than a configuration one.

## 5. Link the status page

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

Next: [Coming from Homer or Homepage](migration.md)
