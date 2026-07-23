# FAQ

**Does cairn phone home, track visitors, or call any API?**
No. The container makes zero outbound requests, unless you explicitly
configure `status.gatus`, in which case it polls that one URL of yours. It
sets one cookie (the language choice). The only third-party requests are icon
slugs, which load from jsdelivr in the visitor's browser; use
[your own files](recipes/icons.md#your-own-files) if you want none.

**Does it need JavaScript?**
No. Pages are fully server-rendered; the only script progressively adds the
search box. Without it, visitors simply see the whole directory.

**Can I put it behind authentication?**
You can (any proxy auth works: Authelia, Tinyauth, basic auth), but cairn is
designed as the *public* front door that explains what's behind the login.
It exposes nothing sensitive: it's your config, rendered.

**How do I add or hide a service?**
Edit the YAML; the page updates within ~2 seconds. There is no cache to bust
and no restart.

**Something broke after an edit, is the site down?**
No. A config error at reload keeps the last good version serving; the log
tells you the file, line and expected shape. Only a bad config at *boot*
stops the server.

**Why is a category heading in the wrong language?**
Headings derive from the `category` id unless you name them in
`categories.yaml`; see the [reference](reference.md#categoriesyaml).

**How big is this?**
One ~12 MB image, one process, a few MB of RAM. `FROM scratch`, non-root, no
shell inside.

**Can visitors switch language permanently?**
Yes: the header switcher (or visiting any locale URL) sets a one-year
cookie; `/` honors it from then on.

**Where are the widgets / weather / graphs?**
Not here, on purpose; see the [comparison](comparison.md). The one
exception is [status dots fed by your Gatus](recipes/gatus.md), polled
server-side, because "is it up?" is a guest question too.
