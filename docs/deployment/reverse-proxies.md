# Reverse proxies

cairn is a plain HTTP service on port 8080; any proxy works with its
defaults. One detail matters: forward `X-Forwarded-Proto`, so `sitemap.xml`
and `robots.txt` emit `https://` URLs. All the proxies below do it out of the
box.

A domain, a subdomain and a sub-subdomain all work with no extra
configuration: `tools.example.org` and `cairn.tools.example.org` are the same
case as far as cairn is concerned. Serving from a **sub-path** of an existing
domain takes one flag, [below](#under-a-sub-path).

## Caddy

```caddy
tools.example.org {
    reverse_proxy cairn:8080
}
```

## Traefik

```yaml
# on the cairn service in compose.yaml
labels:
  - traefik.enable=true
  - traefik.http.routers.cairn.rule=Host(`tools.example.org`)
  - traefik.http.routers.cairn.entrypoints=websecure
  - traefik.http.routers.cairn.tls.certresolver=letsencrypt
  - traefik.http.services.cairn.loadbalancer.server.port=8080
```

## Nginx

```nginx
server {
    server_name tools.example.org;
    listen 443 ssl;

    location / {
        proxy_pass http://cairn:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Pangolin

Add a resource pointing at `http://cairn:8080` (or the host port you
published). No special headers or path rules needed; leave authentication off:
cairn is meant to be the public front door.

## Under a sub-path

Some organisations have one domain and hand out paths rather than subdomains:
`example.org/cairn/` instead of `cairn.example.org`. Tell cairn where it
lives and every URL it writes carries the prefix:

```yaml
# compose.yaml
services:
  cairn:
    image: ghcr.io/morgankryze/cairn:latest
    command: ["-base-path", "/cairn"]
    ports:
      - 8080:8080
    volumes:
      - ./config:/config:ro
```

One rule comes with it, and getting it wrong is silent: `site.yaml`'s `url` is
the **domain alone**, `https://example.org`, not `https://example.org/cairn`.
cairn appends the prefix itself, so writing it twice puts it twice in every
canonical link and sitemap entry. It is a config error rather than something
cairn emits: on a fresh start the site is replaced by the getting-started page
with the reason in the log, and on a reload the previous pages keep serving.
[Site](../configuration/site.md#the-domain-and-nothing-else) has the details.

Worth knowing too: four paths that tools only ever read at the root of a domain
move under the prefix with everything else, and cairn cannot serve what it does
not own. `/robots.txt` is never fetched from anywhere but the root, so its
directives and the `Sitemap:` line it carries go unread; `/favicon.ico` is what
feed readers and link previewers fetch when they skip the html; and RFC 9116
wants `/.well-known/security.txt` at the root too. If any of that matters to
you, alias those paths at the proxy to their prefixed versions. The two probes
are the exception cairn handles itself: `/healthz` and `/readyz` keep answering
at the root, because an orchestrator talks to cairn directly rather than
through the proxy that adds the prefix.

cairn then answers on `/cairn/…` and strips the prefix back off itself, so
**the proxy needs no path rewriting**. Point it at cairn and stop there:

```nginx
location /cairn/ {
    proxy_pass http://cairn:8080;          # no trailing slash: keep the prefix
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

```caddy
example.org {
    handle /cairn/* {
        reverse_proxy cairn:8080           # handle, not handle_path: keep the prefix
    }
}
```

```yaml
# Traefik, on the cairn service in compose.yaml
labels:
  - traefik.enable=true
  - traefik.http.routers.cairn.rule=Host(`example.org`) && PathPrefix(`/cairn`)
  - traefik.http.services.cairn.loadbalancer.server.port=8080
  # no stripprefix middleware: cairn expects the full path
```

Three things worth knowing:

- **The prefix is fixed at startup**, not read from a header. cairn renders
  its pages once per config and serves the bytes, so the mount point has to be
  known when they are built. Restart to change it.
- **`/healthz` also answers at the domain root**, so the container healthcheck
  keeps working unchanged: it talks to cairn directly, not through the proxy.
- **Set `url` in `site.yaml` to the origin only** (`https://example.org`, no
  path). cairn appends the base path itself for canonical links, the sitemap
  and `robots.txt`.

Next: [Icons](../recipes/icons.md)
