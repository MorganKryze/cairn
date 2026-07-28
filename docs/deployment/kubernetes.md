# Kubernetes

cairn happens to fit Kubernetes almost too well: one stateless container, no
database, no persistent volume, no write anywhere. The whole deployment is a
ConfigMap holding your YAML, a Deployment reading it, a Service, and whatever
you already use for ingress.

## The whole thing

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cairn-config
data:
  site.yaml: |
    title: Our tools
    locales: [fr, en]
    about:
      fr: Bienvenue. Voici les services que nous hébergeons.
      en: Welcome. Here are the services we host.
  services.yaml: |
    - id: pad
      url: https://pad.example.org
      icon: hedgedoc
      name: { fr: Bloc-notes, en: Notepad }
      desc: { fr: Écrire à plusieurs., en: Write together. }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cairn
spec:
  replicas: 1
  selector:
    matchLabels: { app: cairn }
  template:
    metadata:
      labels: { app: cairn }
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile: { type: RuntimeDefault }
      containers:
        - name: cairn
          image: ghcr.io/morgankryze/cairn:stable
          ports:
            - containerPort: 8080
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: { drop: ["ALL"] }
          livenessProbe:
            httpGet: { path: /healthz, port: 8080 }
          readinessProbe:
            httpGet: { path: /readyz, port: 8080 }
          resources:
            requests: { cpu: 10m, memory: 16Mi }
            limits: { memory: 64Mi }
          volumeMounts:
            - name: config
              mountPath: /config
              readOnly: true
      volumes:
        - name: config
          configMap: { name: cairn-config }
---
apiVersion: v1
kind: Service
metadata:
  name: cairn
spec:
  selector: { app: cairn }
  ports:
    - port: 80
      targetPort: 8080
```

Add your usual Ingress or HTTPRoute pointing at the `cairn` Service on port 80.
Forward `X-Forwarded-Proto` so `sitemap.xml` and `robots.txt` emit `https://`
URLs; every common controller does it already.

## Why the two probes differ

`livenessProbe` uses `/healthz`, which answers `200` whenever the process is
up, whatever the configuration says. That is deliberate. cairn serves a
getting-started page instead of dying when the config is missing or invalid, so
a liveness probe that failed on a bad config would restart-loop the pod and
undo the point.

`readinessProbe` uses `/readyz`, which answers `503` until a valid config has
loaded. So a pod that came up with an empty ConfigMap is running, visible in
`kubectl get pods`, and correctly kept out of the Service until it has
something real to serve.

## Editing the config

Change the ConfigMap and cairn picks it up. No rollout, no restart.

This is not luck. A ConfigMap mount is not an ordinary directory: kubelet never
edits the files in place, it writes a new timestamped directory and atomically
repoints a `..data` symlink at it. File watchers routinely miss that, which is
exactly why cairn polls its config directory instead of using inotify.

The one thing to expect is the delay. cairn polls every two seconds, but
kubelet only refreshes a mounted ConfigMap on its own sync loop, up to about a
minute. So an edit lands within a minute or so rather than instantly. If a new
config is invalid, cairn logs the file and the line and keeps serving the
previous pages, staying ready: the site is up, just not on your newest edit.

## What does not fit in a ConfigMap

- **Preview images.** A ConfigMap is limited to about 1 MiB and holds text.
  Point `images:` at absolute URLs, or mount a volume at `/config/media`.
- **Your own icons and logo.** Same answer: a second ConfigMap mounted at
  `/assets` works for SVG, a volume for anything heavier. See
  [Icons](../recipes/icons.md).
- **A very large catalogue.** Hundreds of services still fit comfortably in
  text, but if you split across many files, remember the limit applies to the
  ConfigMap as a whole. [Multiple files](../recipes/multiple-files.md) works
  the same way here: every `*.yaml` key in the mount is read.

## Serving from a sub-path

If your Ingress puts cairn under `example.org/cairn/` rather than its own host,
tell cairn where it lives and it carries the prefix through every link,
redirect and asset by itself, so the Ingress needs no rewrite annotation:

```yaml
          args: ["-base-path", "/cairn"]
```

Both probes keep answering at the root, so the manifest above stays valid.
More in [Reverse proxies](reverse-proxies.md#under-a-sub-path).

## Checking a config before it ships

`cairn -check` validates a directory and exits non-zero, which makes it a
natural gate in whatever pipeline renders your manifests:

```sh
docker run --rm -v ./config:/config ghcr.io/morgankryze/cairn:stable -check
```

Next: [Reverse proxies](reverse-proxies.md)
