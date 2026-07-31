# Helm

The [Kubernetes page](kubernetes.md) hands you four objects to paste and edit,
which is honest work right until the Ingress: hostname, TLS secret, class,
annotations, all the parts that differ per cluster and that a manifest printed
in a document cannot know. That is what the chart is for. Everything else it
installs is the same ConfigMap, Deployment and Service, rendered from values
instead of copied by hand.

## Installing

The chart ships as an OCI artifact next to the image, so there is no repository
to add first:

```sh
helm install cairn oci://ghcr.io/morgankryze/charts/cairn
```

That is a running cairn with no config, which is a valid state rather than a
failure: it serves its getting-started page, and readiness deliberately keeps
it out of the Service until you feed it. Look at it now:

```sh
kubectl port-forward svc/cairn 8080:80
```

Add `--version` with the number from the
[releases](https://github.com/MorganKryze/cairn/releases) to pin. Chart version
and cairn version are the same number, always: chart `1.14.0` installs cairn
`1.14.0`, so there is only ever one version to talk about, and `image.tag` is
there if you ever want to break the pair apart.

## Feeding it your config

Your YAML goes under `config:`, one entry per file, exactly the files you would
bind-mount under Docker. They land in a ConfigMap mounted read-only at
`/config`:

```yaml
# values.yaml
config:
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
```

```sh
helm upgrade --install cairn oci://ghcr.io/morgankryze/charts/cairn -f values.yaml
```

The `|` is doing real work there. It makes each block one opaque string, so
your config reaches cairn as written, with its own indentation and its own
`{ fr: …, en: … }` maps, instead of being folded into Helm's values tree where
a key of yours could collide with a key of the chart's. Everything in
[Services](../configuration/services.md) and [Site](../configuration/site.md)
applies unchanged inside those blocks, and
[multiple files](../recipes/multiple-files.md) works the way it does anywhere
else: every `*.yaml` entry is read.

## Editing it later

`helm upgrade` with your edited values updates the ConfigMap, and there it
stops. No rollout, no restart, no pod churn: cairn polls its config directory
and picks the change up by itself, within a minute or so, once kubelet has
refreshed the mounted ConfigMap on its own sync loop.

This is why the chart carries no `checksum/config` annotation, the trick most
charts use to force a restart when a ConfigMap changes. Here it would cost you
a rolling restart to achieve something that already happens.

If the new config is invalid, cairn names the file and the line in its log and
keeps serving the previous pages, staying ready. The site is up, just not on
your newest edit.

## Keeping the ConfigMap out of the release

If your config already lives somewhere else, sealed secrets, a kustomize
overlay, a git-ops repository of its own, point the chart at it and it creates
no ConfigMap at all:

```yaml
existingConfigMap: cairn-config
```

## Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  host: cairn.example.org
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
  tls:
    enabled: true
```

`tls.secretName` defaults to the release's own resource name plus `-tls`, so
`cairn-tls` for a release named `cairn` and `myrelease-cairn-tls` for one named
`myrelease`. Set it yourself if the certificate already exists.

One host and one path is all the chart renders. Serving one instance on several
hostnames is rare enough to deserve its own Ingress object next to the release,
and the Service is there, named after the release, ready to be pointed at.

Forward `X-Forwarded-Proto` so `sitemap.xml` and `robots.txt` emit `https://`
URLs. Every common controller already does.

## Under a sub-path

```yaml
basePath: /cairn
ingress:
  enabled: true
  host: example.org
  path: /cairn
```

Two keys, one value, and they answer different questions: `basePath` tells
cairn where it lives so it carries the prefix through every link, redirect and
asset itself, `ingress.path` tells the controller what to route. Neither side
needs a rewrite rule. Both probes keep answering at the root, so nothing else
in the release moves. More in
[Reverse proxies](reverse-proxies.md#under-a-sub-path).

## Icons, images, anything that is not text

cairn serves `/assets` by default, so whatever you mount there is reachable
with no extra flag. What trips people up is the shape, not the mount: **a
ConfigMap key cannot contain a slash**, so one ConfigMap cannot hold both
`icons/nginx.svg` and `logo.png`. It takes one per directory, mounted at its
own depth:

```sh
kubectl create configmap cairn-icons --from-file=assets/icons/
kubectl create configmap cairn-logo --from-file=logo.png=assets/logo.png
```

```yaml
extraVolumes:
  - name: logo
    configMap:
      name: cairn-logo
  - name: icons
    configMap:
      name: cairn-icons
extraVolumeMounts:
  - name: logo
    mountPath: /assets # gives /assets/logo.png
    readOnly: true
  - name: icons
    mountPath: /assets/icons # gives /assets/icons/*.svg
    readOnly: true
```

Nesting one mount inside the other is allowed, and the deeper one wins for its
own subtree: `/assets/logo.png` comes from the first, `/assets/icons/nginx.svg`
from the second.

Binary files are fine. `kubectl create configmap --from-file` notices a png and
puts it under `binaryData`, base64 encoded, without being told. The limit is
the ConfigMap's, about 1 MiB for the whole object, which is roomy for svg
icons and a logo and too small for a gallery. Preview images under
`/config/media` are the same story: a volume once they grow. See
[Icons](../recipes/icons.md).

## Every value

| Key                                                         | Default                                   | What it does                                                                                       |
| ----------------------------------------------------------- | ----------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `config`                                                    | `{}`                                      | Your YAML, one entry per file, rendered into a ConfigMap                                           |
| `existingConfigMap`                                         | `""`                                      | Use your own ConfigMap instead; `config` is then ignored                                           |
| `basePath`                                                  | `""`                                      | Serve under a sub-path, e.g. `/cairn`                                                              |
| `image.repository` / `image.tag` / `image.pullPolicy`       | ghcr.io image, appVersion, `IfNotPresent` | Which cairn to run                                                                                 |
| `replicaCount`                                              | `1`                                       | Stateless, so more is fine, and rarely necessary                                                   |
| `service.type` / `service.port`                             | `ClusterIP`, `80`                         | The Service in front of the pods                                                                   |
| `ingress.*`                                                 | disabled                                  | `enabled`, `className`, `host`, `path`, `pathType`, `annotations`, `tls.enabled`, `tls.secretName` |
| `resources`                                                 | 10m/16Mi requested, 64Mi limit            | No CPU limit on purpose, so a burst of visitors is not throttled                                   |
| `podSecurityContext` / `securityContext`                    | the hardened set below                    | Change only if your cluster forces it                                                              |
| `extraVolumes` / `extraVolumeMounts`                        | `[]`                                      | Assets, media, anything else on disk                                                               |
| `imagePullSecrets`                                          | `[]`                                      | Credentials for a private registry; the Secrets must already exist in the namespace                |
| `podAnnotations`, `nodeSelector`, `tolerations`, `affinity` | empty                                     | The usual scheduling knobs                                                                         |

## The hardening

The defaults are the ones from the [hardened compose](docker-compose.md):
non-root as 65534, read-only root filesystem, every capability dropped,
`RuntimeDefault` seccomp. cairn writes nowhere and needs none of what it gives
up.

They are values rather than hardcoded for one reason: OpenShift assigns its own
UID range and rejects a fixed `runAsUser`. Set it to `null` there and let the
platform choose, the image runs under any UID. Anywhere else, leaving this
block alone is the right move.

## From a registry, through Argo CD

Nothing here assumes you run `helm install` by hand. The chart is an ordinary
OCI artifact, so it can be mirrored into whatever registry you already keep, and
driven from there:

```sh
helm pull oci://ghcr.io/morgankryze/charts/cairn --version 1.14.0
helm push cairn-1.14.0.tgz oci://harbor.internal/helm
```

Argo CD needs the repository declared before an Application can name it. Note
the `url` carries no `oci://` prefix in this field:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: harbor-helm
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: harbor-helm
  type: helm
  url: harbor.internal/helm
  enableOCI: "true"
```

Then the Application. `targetRevision` is the chart version, and since the chart
version is the cairn version, that one field is the version of cairn you are
pinning: bumping it is the whole upgrade.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cairn
  namespace: argocd
spec:
  project: default
  source:
    repoURL: harbor.internal/helm
    chart: cairn
    targetRevision: 1.14.0
    helm:
      valuesObject:
        ingress:
          enabled: true
          className: nginx
          host: tools.internal
        config:
          site.yaml: |
            title: Our tools
            locales: [fr, en]
  destination:
    server: https://kubernetes.default.svc
    namespace: cairn
```

Once the values outgrow an inline block, keep them in a repository of their own
and reference it:

```yaml
spec:
  sources:
    - repoURL: harbor.internal/helm
      chart: cairn
      targetRevision: 1.14.0
      helm:
        valueFiles:
          - $values/cairn/values.yaml
    - repoURL: https://git.internal/infra/values.git
      targetRevision: main
      ref: values
```

The second source has no `path`, so it generates nothing. It is there only to
make its files reachable as `$values`.

Recent Argo CD versions also accept a source of type `oci`, where `repoURL` does
carry the `oci://` prefix and `path` is `"."`. Both forms install the same chart;
the one above is the one that works on the widest range of versions.

A last thing worth knowing here, because it looks like drift and is not: editing
your config in git changes the ConfigMap, Argo CD syncs it, and no pod restarts.
cairn reloads on its own, so the Application settles as Synced and Healthy with
nothing rolled.

## Checking the chart came from here

The chart is signed the same way the image is, keylessly, bound to the workflow
that published it:

```sh
cosign verify ghcr.io/morgankryze/charts/cairn:1.14.0 \
  --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

More in [SECURITY.md](../../SECURITY.md#verifying-what-you-pulled).

## What the chart deliberately leaves out

No HorizontalPodAutoscaler: a page rendered from memory by a process that idles
near zero is not the thing that falls over. No ServiceAccount: cairn talks to no
API, so the default one it never uses is enough. No PersistentVolumeClaim, no
PodDisruptionBudget, no CRD, nothing to back up.

If you need something the values do not cover, take the manifests and own them:

```sh
helm template cairn oci://ghcr.io/morgankryze/charts/cairn -f values.yaml > cairn.yaml
```

That prints exactly what would be installed, which is also the honest way to
review a chart before trusting it.

## Uninstalling

```sh
helm uninstall cairn
```

Everything goes with it. cairn stores nothing, so there is no volume to reclaim
and no leftover to hunt down.

Next: [Air-gapped](airgap.md)
