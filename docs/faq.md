# FAQ

**Does cairn phone home, track visitors, or call any API?**
No. The container makes zero outbound requests, unless you explicitly
configure `status.gatus`, in which case it polls that one URL of yours. It
sets at most two first-party cookies, both functional: the language choice
(only when a visitor uses the switcher) and the dismissed welcome note (only
if you configure `about`). Strictly functional cookies like these are what
the ePrivacy consent exemption covers: no banner needed, and the demo's
privacy page shows how to say so. The only third-party requests are icon
slugs, which load from jsdelivr in the visitor's browser;
[two commands remove even that](recipes/icons.md#going-fully-self-hosted).

**Can I see how many people visit?**
Not from cairn, ever: no analytics is part of the promise the page makes to
its visitors. Your reverse proxy already writes an access log; if you need
numbers, count there.

**Does it need JavaScript?**
No. Pages are fully server-rendered; small scripts add the search box, the
theme toggle, the welcome note's dismiss button and the highlight that follows
you down the category trail. Without them the directory is all still there,
every category, every card, every link, in the system's theme. On a phone the
trail stays a row of chips instead of folding into the compact jump-to select,
which is a control only a script can drive.

**Can I put it behind authentication?**
You can (any proxy auth works: Authelia, Tinyauth, basic auth), but cairn is
designed as the _public_ front door that explains what's behind the login.
It exposes nothing sensitive: it's your config, rendered.

**How do I add or hide a service?**
Edit the YAML; the page updates within ~2 seconds. There is no cache to bust
and no restart.

**Something broke after an edit, is the site down?**
No. A config error at reload keeps the last good version serving; the log
tells you the file, line and expected shape. Even a bad config at _boot_
serves a getting-started page rather than a dead container, and your site
takes over the moment the config is valid.

**Why is a category heading in the wrong language?**
Headings derive from the `category` id unless you name them in
`categories.yaml`; see the [reference](reference.md#categoriesyaml).

**How big is this?**
One image: 4.3 MB to pull, about 10 MB unpacked, nearly all of that the static
binary. One process, a few MB of RAM. `FROM scratch`, non-root, no shell
inside.

**Can visitors switch language permanently?**
Yes: the header switcher sets a one-year cookie and `/` honors it from
then on. Only the switcher sets it, so visitors who never touch it simply
follow their browser language.

**Where are the widgets / weather / graphs?**
Not here, on purpose; see the [comparison](comparison.md). The one
exception is [status dots fed by your monitor](recipes/status.md), polled
server-side, because "is it up?" is a guest question too.

**Does it work with Uptime Kuma?**
Yes: [`status.provider: kuma`](recipes/status.md#uptime-kuma).
This page used to say no, on the grounds that Kuma keeps a database, sits
behind authentication and is configured by clicking, and that supporting it
would import all of that. Reading a **published status page** imports none of
it: two unauthenticated GETs, no login, no database of cairn's own.

What has not changed is that there is no `cairn -emit-kuma` and there cannot
be one. Kuma has no config file and no setup API, so a Kuma user names each
monitor after the cairn service id by hand. That is measured rather than
assumed: standing up the instance this was tested against took a script
driving a browser.

Anything else with a status API is read too, through
[`status.provider: json`](recipes/status.md#any-other-status-api) and a
mapping. Statuspage, Instatus, Upptime, Cachet, UptimeRobot and Better Stack
have each been read from a live instance;
[the full list](recipes/status.md#which-monitors-cairn-reads) says which
monitors were run, which should work, and which cannot be read at all.

Gatus is still the one cairn writes the config for, and still the only monitor
that can point a pill at a page of its own per service. A quiet bonus of
either pairing: cairn shows the status labels in your visitors' language,
which neither monitor's own UI does.

**Can each guest get their own view of the page?**
No. Per-guest views need accounts, and accounts are the dashboard road
cairn deliberately does not take. Everyone sees the same calm page; if two
audiences really differ, run two cairns with two config folders.
