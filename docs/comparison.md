# Comparison

If you already run a dashboard, cairn is probably not a replacement for it.

Every project on this page draws a grid of links in cards, so the family
resemblance is real. The difference is who the grid is drawn for. Homer,
Homepage, Dashy and the rest are built for the person who runs the server:
dense, informative, tuned to an operator's eye. cairn is built for the people
that person hosts services *for*, who did not choose the stack, do not know
what a container is, and mostly want to know what this place is and whether
the thing they need works right now.

Which is why plenty of setups run both: Homepage on the private side for you,
cairn on the public side for everyone else. They are not competing for the
same tab.

## Pick by audience

| You want | Use |
| --- | --- |
| A page your **guests** read: plain language, their language, "what is this and when do I use it" | **cairn** |
| A fast personal tile board, huge theme ecosystem | [Homer](https://github.com/bastienwirtz/homer) |
| Widgets and live integrations (Docker, \*arr, monitoring) on your own dashboard | [Homepage](https://github.com/gethomepage/homepage) |
| A feeds-and-widgets start page (RSS, weather, markets) | [Glance](https://github.com/glanceapp/glance) |
| A dashboard with a built-in config UI and auth | [Dashy](https://github.com/Lissy93/dashy) |
| The most minimal start page with pinned apps | [Flame](https://github.com/pawelmalak/flame) |
| A portal with per-user tabs wrapped around Plex and the \*arrs | [Organizr](https://github.com/causefx/Organizr) |
| Shared boards for a household or team, edited in a GUI, with accounts | [Homarr](https://github.com/homarr-labs/homarr) |

## What cairn does that a dashboard does not

- **It writes for guests.** Descriptions answer "what does this do for me",
  optional detail pages answer "when would I use it". No uptime percentages,
  no CPU graphs: the people reading do not run the servers, and numbers they
  cannot act on are noise.
- **It speaks their language, structurally.** One config carries every
  translation inline, the server negotiates from the browser, a switcher pins
  the choice, and each locale gets its own canonical and `hreflang`. Seven
  interface languages ship built in, and right-to-left scripts lay out
  properly rather than being bolted on.
- **It serves the legal pages.** Legal notice, privacy, anything else you
  need, from the same YAML. Self-hosters, and associations especially, are
  required to publish these and usually have nowhere to put them.
- **It says where each service lives.** An optional flag marks a tool as
  self-hosted or external, so a visitor can see at a glance which links keep
  their data on your machine and which hand it to someone else. That is a
  trust question, and no dashboard answers it because operators already know.
- **It can make no third-party request at all.** Icons are served from your
  own files if you want them to be, there is no CDN font, no analytics, no
  outbound call except the one Gatus URL you configure. The
  [demo](../demo/README.md) runs on a network with no route out to prove it.
- **It is a sub-10 MB `FROM scratch` image**, non-root, no shell, no interpreter,
  one Go dependency. The image is signed, ships SLSA provenance and an SBOM,
  and the binaries are attested. That matters when the page is the one thing
  you expose publicly.
- **It reads like a page, not a control panel.** A hero that says what this
  place is, typography meant for prose, contrast measured against WCAG AA, and
  every feature still working with JavaScript turned off.

## What the others do that cairn never will

Worth knowing now rather than discovering after you have migrated.

- **No widgets, no integrations.** cairn will not show download speeds,
  container states or calendar events. That is permanent, not a roadmap item.
- **No auth, no users, no admin UI.** The config is a file you mount read
  only; the page is public by design. Nothing to log into, nothing to lock.
- **No per-guest views**, which are accounts wearing a costume, and **no
  analytics**, because your reverse proxy log already counts visits. Organizr
  and Homarr do these well precisely because they embraced users and state.
  cairn stays the page before the login.
- **No Docker socket**, so nothing is auto-discovered. You write the services
  you want shown, which is a chore the others spare you and a permission cairn
  never needs.
- If the audience is *you*, Homer or Homepage will make you happier. Admin
  dashboards are their home turf, and cairn's guest features would only get in
  your way.

## Coming from one of them

Homer and Homepage configs map over field by field: see
[Migration](recipes/migration.md). You do not have to choose, and most people
who adopt cairn keep the dashboard they already had.
