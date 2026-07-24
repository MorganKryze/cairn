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
No. Pages are fully server-rendered; small scripts progressively add the
search box, the category trail, the theme toggle and the welcome note's
dismiss button. Without them, visitors simply see the whole directory in
their system's theme.

**Can I put it behind authentication?**
You can (any proxy auth works: Authelia, Tinyauth, basic auth), but cairn is
designed as the *public* front door that explains what's behind the login.
It exposes nothing sensitive: it's your config, rendered.

**How do I add or hide a service?**
Edit the YAML; the page updates within ~2 seconds. There is no cache to bust
and no restart.

**Something broke after an edit, is the site down?**
No. A config error at reload keeps the last good version serving; the log
tells you the file, line and expected shape. Even a bad config at *boot*
serves a getting-started page rather than a dead container, and your site
takes over the moment the config is valid.

**Why is a category heading in the wrong language?**
Headings derive from the `category` id unless you name them in
`categories.yaml`; see the [reference](reference.md#categoriesyaml).

**How big is this?**
One ~14 MB image, one process, a few MB of RAM. `FROM scratch`, non-root, no
shell inside.

**Can visitors switch language permanently?**
Yes: the header switcher sets a one-year cookie and `/` honors it from
then on. Only the switcher sets it, so visitors who never touch it simply
follow their browser language.

**Where are the widgets / weather / graphs?**
Not here, on purpose; see the [comparison](comparison.md). The one
exception is [status dots fed by your Gatus](recipes/gatus.md), polled
server-side, because "is it up?" is a guest question too.

**Why Gatus and not Uptime Kuma?**
Because Gatus matches cairn's own shape: one YAML file, stateless, no
login, and [`cairn -emit-gatus`](recipes/gatus.md) writes its config for
you. Uptime Kuma is a fine monitor, but it is heavier, keeps a database,
sits behind authentication and is configured by clicking; supporting it
would import all of that. This is a decision, not a backlog item, and it
holds for other monitoring stacks too. A quiet bonus of the pairing: cairn
shows the status labels in your visitors' language, which the Gatus UI
itself does not do.

**Can each guest get their own view of the page?**
No. Per-guest views need accounts, and accounts are the dashboard road
cairn deliberately does not take. Everyone sees the same calm page; if two
audiences really differ, run two cairns with two config folders.
