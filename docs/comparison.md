# Comparison

Honest guide. These are good projects; pick by audience, not by feature
count.

| You want                                          | Use          |
| ------------------------------------------------- | ------------ |
| A page your **guests** read: plain language, their language, "what is this and when do I use it" | **cairn**    |
| A fast personal tile board, huge theme ecosystem  | [Homer](https://github.com/bastienwirtz/homer) |
| Widgets and live integrations (Docker, \*arr, monitoring) on your own dashboard | [Homepage](https://github.com/gethomepage/homepage) |
| A feeds-and-widgets start page (RSS, weather, markets) | [Glance](https://github.com/glanceapp/glance) |
| A dashboard with a built-in config UI and auth    | [Dashy](https://github.com/Lissy93/dashy) |
| The most minimal start page with pinned apps      | [Flame](https://github.com/pawelmalak/flame) |

Where cairn stands apart:

- **Guest-first content.** Descriptions answer "what does this do for me",
  detail pages answer "when would I use it". No uptime numbers, no CPU
  graphs: your visitors don't run the servers.
- **Structural i18n.** One config with translations inline; language
  detection, switcher, per-locale SEO. The others need one config per
  language, if that.
- **Editorial layout.** A hero that says what the place is, typography meant
  for reading, not a wall of tiles.

Where the others win, so you don't discover it late:

- cairn has **no widgets and no integrations**: it will not show download
  speeds, container states or calendar events, and that's permanent.
- **No auth, no users, no admin UI**: config is a mounted file, the page is
  public by design.
- If the audience is *you*, Homer or Homepage will make you happier; admin
  dashboards are their home turf, and cairn's guest features would just be in
  your way.

All of the above share the deployment model: one container, a YAML file, no
database.
