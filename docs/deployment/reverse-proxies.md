# Reverse proxies

cairn is a plain HTTP service on port 8080; any proxy works with its
defaults. One detail matters: forward `X-Forwarded-Proto`, so `sitemap.xml`
and `robots.txt` emit `https://` URLs. All the proxies below do it out of the
box.

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

Next: [Icons](../recipes/icons.md)
