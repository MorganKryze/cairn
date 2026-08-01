# Status page

cairn tells visitors what exists; a monitor tells them what is up. cairn reads
the monitor you already run, server-side. It never probes your services itself,
and never asks a visitor's browser to.

## Pick your monitor

| you run                                                | write this                                           | you get                                                       |
| ------------------------------------------------------ | ---------------------------------------------------- | ------------------------------------------------------------- |
| [Gatus](https://github.com/TwiN/gatus)                 | `status.gatus`                                       | a pill per service, each linking to that service's own page   |
| [Uptime Kuma](https://github.com/louislam/uptime-kuma) | `status.url`, `status.provider: kuma`, `status.slug` | a pill per service, all linking to the published status page  |
| anything else with a status API                        | `status.url`, `status.provider: json`, `status.map`  | the same, from any document shaped like a list of name, state |

Whichever you pick, the agreement is the same: **the monitor's name for a
service is the cairn service id**. Nothing else has to match.

Gatus first below, because it is the one cairn can write the config for.
[Which monitors cairn reads](#which-monitors-cairn-reads) at the end of this
page says what has been read from a live instance, what should work, and what
cannot be read at all.

## Gatus

```yaml
# site.yaml
status:
  gatus: https://status.example.org # base URL of your Gatus
  interval: 60s # optional; default 60s, minimum 5s
```

That is the whole integration. cairn polls
`{gatus}/api/v1/endpoints/statuses`, matches **endpoint name to service id**,
and links each pill to the endpoint page Gatus reports for it, so your
endpoints can be grouped however you like or not grouped at all.

### If your visitors cannot reach that address

An internal address such as `http://gatus:8080` resolves between containers and
nowhere else. Say where the pill should point:

```yaml
status:
  gatus: http://gatus:8080 # internal: cairn polls this, server-side
  page: https://status.example.org # public: the pill links visitors here
```

Or say that there is no public one at all:

```yaml
status:
  gatus: http://gatus:8080
  linked: false # the pills state the status, nothing more
```

`linked: false` keeps the dot and its label and drops the link. The pills stop
being controls: no target, no keyboard stop, and on a card a click goes where
the rest of the card goes, to the service itself.

### Let cairn write the endpoints

One endpoint per service with an `http(s)` url (path-only urls are skipped),
named after its id, grouped by category:

```sh
docker run --rm -v ./config:/config:ro <your-cairn-image> -emit-gatus > gatus/endpoints.yaml
```

```yaml
endpoints:
  - name: pdf
    group: documents
    url: https://pdf.example.org
    interval: 5m # how often Gatus probes the service
    conditions:
      - "[STATUS] == 200"
```

Merge it into your Gatus config; alerting stays yours. Adjust the conditions
per service if 200 is not the right health signal.

### Keep the history across restarts

Gatus stores results in memory by default and loses them when it restarts. That
shows on cairn: with nothing stored, the API answers with endpoints and no
results, so every pill reads Unknown until the first check of each lands.

```yaml
storage:
  type: sqlite
  path: /data/data.db # on a volume, or the file goes with the container
```

`postgres` is the other option, with a connection URL in the same `path`.

### Hide what Gatus probes

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

`hide-url` redacts the address inside the error text too, which is the leak
worth knowing about: it only shows when a check fails, so it survives a test on
a healthy endpoint. The port outlives the other two keys, hence `hide-port`.
Gatus also has `hide-errors`, which drops the message instead of redacting it.

None of this reaches cairn: the pills come from the name, the key and whether
the last check passed. `cairn -emit-gatus -hide-targets` writes the block on
every endpoint, so regenerating the file does not lose it.

### A certificate cairn does not know

Usual on an internal network: every pill reads Unknown and the log names an
`x509` failure. Give cairn the authority that signed it.

```yaml
status:
  gatus: https://status.internal
  ca: /assets/ca.crt # a file in the mounted assets directory
  # ca: https://pki.internal/ca.crt  # or an address cairn fetches it from
```

The bundle is **added** to the roots cairn already trusts, not swapped for
them, so a publicly signed Gatus keeps working and adding a bundle narrows
nothing else. It is read once and cached, so a rotated authority needs a
restart, like a mounted one.

An `http://` URL is allowed, because an internal PKI often has nowhere better
to serve from and a bundle cannot be fetched over a certificate nobody trusts
yet. It is not free: over http, whatever sits on the path to that address
decides what cairn trusts for the poll, which is where `status.insecure` lands
too. cairn says which of the two it is at startup and on every `-check`.

`status.insecure: true` turns the check off rather than satisfying it. Both
sides of the connection, and the symmetric fix for the certificates Gatus
itself has to trust, are in
[Air-gapped](../deployment/airgap.md#6-self-signed-certificates).

## Uptime Kuma

Kuma publishes a **status page**, and that page answers two endpoints with no
login. cairn reads those.

```yaml
# site.yaml
status:
  provider: kuma
  url: https://kuma.example.org # the Kuma instance
  slug: tools # the published status page, from its own URL
  interval: 60s
```

The slug is the last part of the page's URL:
`https://kuma.example.org/status/tools` is `tools`. Kuma serves statuses per
status page rather than per instance, so the slug is half the address, and
cairn refuses to start without it rather than polling an endpoint that cannot
exist.

Three things have to be true, and only the first is work:

1. **Each monitor is named after the cairn service id.** Kuma has no config
   file and no setup API, so there is no `cairn -emit-kuma` and there cannot
   be one: everything in Kuma is created by clicking, which is exactly how the
   instance this was tested against was set up, with a script driving a
   browser.
2. **The monitors are attached to the status page**, inside a group. A page
   with none answers with no monitor, and cairn says so with the slug in the
   message.
3. **The status page is published.** An unpublished one 404s exactly as a
   misspelled slug does, which is why the error names both.

What differs from Gatus:

- **The pill links to the status page**, the same one for every service, since
  Kuma has no page per monitor. Without `status.page`, cairn derives it from
  the address and the slug, which is the URL Kuma itself serves it at.
- **Maintenance shows.** Kuma reports it, so a monitor under maintenance gets
  the blue square pill rather than a red dot. It reports no degraded, and cairn
  invents none.
- **Pending reads as Unknown.** A monitor waiting for its first check has said
  nothing yet, which is what the neutral pill already means.

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
    key: name # field of each element holding the service name
    state: status # field holding its state
    up: [operational]
    degraded: [degraded_performance, partial_outage]
    maintenance: [under_maintenance]
```

A path is a dotted walk and nothing more: `attributes.status` reaches inside a
nested object, and leaving `list` out means the document is itself the array.
No wildcards and no filters, because anything cleverer is a query language, and
a query language in YAML is a second product.

A state can be a word, a number or a boolean: monitors use all three, and
`up: [1]` or `up: [true]` read as you would expect.

The value lists are **allow-lists**: a state in none of them reads as down.
That is deliberate in both directions. A vendor that adds a word next year
cannot make a broken service look green, and an operator who forgot to list one
sees a red pill rather than a wrong one.

`unknown` is the exception, read before the rest. Put a monitor's paused state
there and the service gets no pill at all instead of a red one, which is what
cairn's neutral state has always meant.

### Mappings that were run against the real thing

Read by cairn itself on 2026-08-01, against that live service. Names are the
agreement: a pill is drawn only where a name matches a cairn service id, so on
your own status page, name the components after your service ids.

| service                   | endpoint                                        | `list`         | `key`                           | `state`             | `up`          |
| ------------------------- | ----------------------------------------------- | -------------- | ------------------------------- | ------------------- | ------------- |
| Atlassian Statuspage      | `/api/v2/components.json`                       | `components`   | `name`                          | `status`            | `operational` |
| Instatus                  | `/v2/components.json`                           | `components`   | `name`                          | `status`            | `OPERATIONAL` |
| Upptime                   | `history/summary.json` in the repo              | (none)         | `slug`                          | `status`            | `up`          |
| Better Stack, public page | `/index.json`                                   | `included`     | `attributes.public_name`        | `attributes.status` | `operational` |
| UptimeRobot, public page  | `stats.uptimerobot.com/api/getMonitorList/<id>` | `psp.monitors` | `name`                          | `statusClass`       | `success`     |
| Cachet                    | `/api/v1/components?per_page=100`               | `data`         | `name`                          | `status`            | `1`           |
| Statping-ng               | `/api/services`                                 | (none)         | `name`                          | `online`            | `true`        |
| UptimeRobot API v3        | `/v3/monitors`                                  | `data`         | `friendlyName`                  | `status`            | `UP`          |
| Better Stack API v2       | `/api/v2/monitors`                              | `data`         | `attributes.pronounceable_name` | `attributes.status` | `up`          |
| StatusCake API v1         | `/v1/uptime?limit=100`                          | `data`         | `name`                          | `status`            | `up`          |

Notes worth having before you write yours:

- **Statuspage's other states** are `degraded_performance` and `partial_outage`
  for degraded, `under_maintenance` for maintenance. Cloudflare's page had 34
  and 20 of those while this was written, so the two newer pills are not
  theoretical.
- **UptimeRobot** calls a switched-off monitor `paused`. Put it in `unknown`,
  or five paused monitors read as five outages.
- **Cachet** paginates at 20, hence `?per_page=100`. Its `status` is a number
  (1 operational, 2 performance issues, 3 partial outage, 4 major outage);
  `status_name` is the readable twin, but its wording follows the language of
  that instance, so the number travels better.
- **The three token APIs** each name their own vocabulary, so their `unknown`
  lists are worth copying: UptimeRobot answers `PAUSED, STARTED, UP,
LOOKS_DOWN, DOWN` (it says so itself if you ask for a status it does not
  know), and paused and started both belong in `unknown`. Better Stack answers
  `up, down, paused, pending, maintenance, validating`: `validating` is the
  amber one, `paused` and `pending` the quiet ones.
- **StatusCake keeps paused in its own field**, a boolean beside `status`,
  where no mapping can reach it: a paused test keeps whatever state it had. It
  also pages at 25, hence `?limit=100`.
- **A monitor's name has to be a valid cairn service id**: lowercase letters,
  digits and dashes. A monitor named after a domain, `libresoftware.cloud`,
  never matches, because a service id carries no dot. Rename the monitor, or
  the pill never appears.

### The token, if the API needs one

Never in `site.yaml`. cairn takes the **path of a file** holding it:

```yaml
status:
  provider: json
  url: https://uptime.betterstack.com/api/v2/monitors
  token_file: /run/secrets/status-token # a mounted file, not a value
  token_scheme: Bearer # the default; OAuth for Statuspage, Basic for HTTP Basic
  map: { list: data, key: attributes.pronounceable_name, state: attributes.status, up: [up] }
```

A Kubernetes Secret, a docker compose secret and a Vault agent all deliver
exactly that: a file. cairn reads it on every poll, so a rotated token takes
effect without a restart, and it travels in an `Authorization` header, never in
the URL. A failed poll names the key rather than printing the credential.

For an API that wants HTTP Basic, put `base64(user:password)` in the file and
set `token_scheme: Basic`.

The file may not live under `/assets`. That directory is served to every
visitor, so a token there would be published rather than stored, and cairn
refuses to start. `status.ca` under `/assets` stays legal, and the asymmetry is
the point: a CA certificate is the public half of an authority, and a token is
not the public half of anything.

## What the pills say

Same on every monitor, since the states arrive translated into cairn's own
four before anything is drawn.

| pill            | look                       | meaning                           |
| --------------- | -------------------------- | --------------------------------- |
| **Online**      | green, breathing slowly    | it works                          |
| **Degraded**    | amber                      | it works, but not well            |
| **Maintenance** | blue, and square not round | off on purpose, and it comes back |
| **Offline**     | red, static and outlined   | it does not work                  |
| **Unknown**     | neutral, hollow            | nobody has said anything about it |

Every label is localized in all seven built-in languages, and every dot was
measured against the pill it sits in on both themes, because colour alone is
never the only cue. The pulse stops under `prefers-reduced-motion`.

Gatus lights two of them and no more: its result carries a pass or a fail and
nothing else. The other monitors reach all five.

How they behave:

- Until the monitor has answered once, at boot or while it is unreachable,
  every pill reads Unknown rather than stale or absent, and the log says why,
  once, rather than repeating itself every interval.
- Once it has answered, a service it says nothing about shows no pill at all.
- **An open page keeps up.** Start cairn before the monitor and every pill
  reads Unknown; when the monitor comes up, a tab already open catches up on
  its own, with no reload. It refetches the page it is on at `status.interval`
  and swaps only the pills: no separate API, and the request goes to cairn,
  never to the monitor. A background tab does not poll, a pill your keyboard is
  on is left alone until you move off it, and a site with no status ships no
  script at all.

## Which monitors cairn reads

Split by how you get them: something you run yourself, or something somebody
else hosts for you.

### Read against a live instance

Read by cairn itself on 2026-08-01; the count is what came back. The three
marked "+ token" were read from real accounts on their free tiers, with the
credential in a `token_file`.

**Open source, self-hosted**

| monitor             | how              | what was read                                               |
| ------------------- | ---------------- | ----------------------------------------------------------- |
| Gatus               | `status.gatus`   | 5 services on the demo stack, 4 up and 1 down               |
| Uptime Kuma 1.23.17 | `provider: kuma` | 2 monitors on a local instance, one up and one down         |
| Cachet              | `provider: json` | 31 components on status.framasoft.org, 6 on the Cachet demo |
| Statping-ng         | `provider: json` | 6 services on a local instance, 5 up and 1 down             |
| Upptime             | `provider: json` | 4 sites (it runs on GitHub Actions, so there is no server)  |

**Hosted**

| service                          | how                      | what was read                                             |
| -------------------------------- | ------------------------ | --------------------------------------------------------- |
| Atlassian Statuspage             | `provider: json`         | 471 components on Cloudflare, 33 on Discord, 12 on GitHub |
| Instatus                         | `provider: json`         | 1 component                                               |
| UptimeRobot, public status page  | `provider: json`         | 15 monitors, 5 of them paused                             |
| UptimeRobot API v3               | `provider: json` + token | 1 monitor on a real account                               |
| Better Stack, public status page | `provider: json`         | 4 resources, no credential needed                         |
| Better Stack API v2              | `provider: json` + token | 2 monitors on a real account                              |
| StatusCake API v1                | `provider: json` + token | 1 test on a real account, reported down                   |

Anything answering in the Statuspage shape is read by the same mapping even
when Atlassian is nowhere near it: Tailscale's own page, 11 components, was
read with that row unchanged.

### Should work, not run here

The shape or the credential fits, but there was no instance to point cairn at.

**Open source, self-hosted**

| monitor | why it should work                                                             | unverified                      |
| ------- | ------------------------------------------------------------------------------ | ------------------------------- |
| Kener   | `/api/monitors` refuses a missing authorization header, so the credential fits | the shape, and it needs a token |

**Hosted**

| service                                 | why it should work                                | unverified                    |
| --------------------------------------- | ------------------------------------------------- | ----------------------------- |
| Freshstatus, Statuspal, Hund, Status.io | each documents a JSON endpoint listing components | everything: no instance found |

### Cannot be read, and why

**Open source, self-hosted**

| monitor                    | why not                                                                                              |
| -------------------------- | ---------------------------------------------------------------------------------------------------- |
| OpenStatus                 | the key travels in `x-openstatus-key`; cairn sends `Authorization` and no other header               |
| Healthchecks               | the same, in `X-Api-Key`                                                                             |
| Vigil                      | answers one word for the whole page (`healthy`), not a list, so there is nothing to draw per service |
| Netdata                    | answers alarms keyed by name rather than a list, and an alarm is not a service                       |
| Prometheus, Zabbix, Icinga | a metrics format or an RPC, and a different model altogether                                         |
| cState, tinystatus         | static generators: they build a page, not an API                                                     |

**Hosted**

| service            | why not                                                                                                                 |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| HetrixTools        | the token travels in the URL path, and cairn logs a failed poll with the address it called, so the log would publish it |
| Site24x7           | needs an OAuth refresh-token exchange, which would make cairn hold rotating state                                       |
| UptimeRobot API v2 | the key goes in the body of a `POST`; cairn makes one `GET`. Use v3 or the public status page                           |

Two more that are nobody's fault. A **Kuma status page that is not published**
404s exactly as a misspelled slug does, and a **status page that is only HTML**
has nothing to read: several vendors serve one JSON document for the page and
none per component.

## Link the status page

```yaml
# site.yaml
footer:
  - label: { fr: Statut des services, en: Service status }
    url: https://status.example.org
```

Next: [Coming from Homer or Homepage](migration.md)
