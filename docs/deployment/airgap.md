# Air-gapped

cairn asks less of a disconnected network than almost anything else you will
install on it: no database, no outbound call of its own, one static binary in an
image with no shell. The deployment is the four objects the [chart](helm.md)
installs, and they never phone home.

The one thing that does reach the internet is not the server at all. It is your
visitor's browser, fetching icon slugs from a CDN. That is the whole difficulty
of this page, and it is the part that fails quietly: the pods stay healthy, the
page renders, and the icons are simply missing.

## What crosses

| Artifact             | Size     | Where it comes from                                            |
| -------------------- | -------- | -------------------------------------------------------------- |
| `cairn-1.12.0.tgz`   | 4 KB     | `helm pull`, the signed artifact rather than the folder in git |
| `gatus-1.5.0.tgz`    | 9 KB     | the Gatus project's own chart                                  |
| `images.tar`         | 26 MB    | `docker save` of cairn and Gatus                               |
| `assets/icons/*.svg` | a few KB | what `cairn -emit-icons` downloads                             |
| your values          | a few KB | one file per chart, and they are yours to keep                 |

The 26 MB is measured on arm64; amd64 lands in the same range. Almost all of it
is Gatus: cairn's own image is under 14 MB.

## 1. On the connected side

### The chart, verified before it crosses

The order matters. `cosign verify` queries the public transparency log, so it is
not something you get to do on the other side.

```sh
helm pull oci://ghcr.io/morgankryze/charts/cairn --version 1.12.0

cosign verify ghcr.io/morgankryze/charts/cairn:1.12.0 \
  --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The same two lines for `ghcr.io/morgankryze/cairn:1.12.0`, the image. Keep the
output: this is the only moment where you can prove where these bytes came from.

Gatus publishes its own chart, from a classic repository rather than a registry,
so it is fetched differently and travels the same way:

```sh
helm repo add twin https://twin.github.io/helm-charts
helm pull twin/gatus --version 1.5.0
```

That chart versions itself independently of the application it installs, which
is worth knowing before you trust its number: `1.5.0` ships `appVersion v5.34.0`
while Gatus is further along. Pin the image yourself rather than inherit it,
which the values below do.

### The images, pinned by platform

`docker save` exports the platform you pulled and nothing else. Preparing an
amd64 cluster from an arm64 laptop is the classic way to learn this at
`CrashLoopBackOff` rather than at `docker pull`.

```sh
PLATFORM=linux/amd64

docker pull --platform $PLATFORM ghcr.io/morgankryze/cairn:1.12.0
docker pull --platform $PLATFORM ghcr.io/twin/gatus:v5.36.0
```

Pin Gatus to a version rather than `latest`: once the line is cut, `latest`
names nothing you can reason about six months later. To be exact, v5.36.0 is
`sha256:8df96411…` on amd64 and `sha256:5b52c2cb…` on arm64.

Then give both the name they will carry inside, and save them together:

```sh
docker tag ghcr.io/morgankryze/cairn:1.12.0 harbor.internal/cairn:1.12.0
docker tag ghcr.io/twin/gatus:v5.36.0 harbor.internal/gatus:5.36.0
docker save harbor.internal/cairn:1.12.0 harbor.internal/gatus:5.36.0 -o images.tar
```

Retag even if you have no registry. An image named after a host that resolves
nowhere cannot be quietly pulled from the internet by a cluster that turns out
to have a route after all: the mistake becomes loud instead of invisible.

### The icons

This is the step nothing downstream will remind you about.

```sh
docker run --rm -v ./config:/config ghcr.io/morgankryze/cairn:1.12.0 -emit-icons > get-icons.sh
mkdir -p assets && (cd assets && sh ../get-icons.sh)
```

One `assets/icons/<slug>.svg` per slug you use. From then on cairn resolves a
slug to your local file instead of the CDN, with no YAML to change. Preview
images under `images:` follow the same rule: an absolute URL will not load, so
bring the files. See [Icons](../recipes/icons.md).

### The Gatus endpoints

```sh
docker run --rm -v ./config:/config ghcr.io/morgankryze/cairn:1.12.0 -emit-gatus > gatus-endpoints.yaml
```

One endpoint per service, named after its id, which is how each pill finds its
service. More in [Status page](../recipes/gatus.md).

## 2. The check that says whether you are actually ready

```sh
docker run --rm -v ./config:/config:ro -v ./assets:/assets:ro \
  ghcr.io/morgankryze/cairn:1.12.0 -check
```

**Mount both directories.** Given only `/config`, `-check` cannot see the icons
you just downloaded and warns about every one of them, every time, which is
precisely how a warning stops being read. With both, the difference is the whole
point of the exercise:

```console
$ docker run --rm -v ./config:/config:ro … -check
warning: 2 icons load from a CDN in visitors' browsers (gatus, nginx); run cairn -emit-icons to self-host them
ok: 2 services, 1 categories, 0 pages, locales [fr]

$ docker run --rm -v ./config:/config:ro -v ./assets:/assets:ro … -check
ok: 2 services, 1 categories, 0 pages, locales [fr]
```

As long as that line names an icon, your air gap has a hole in it. Once it is
clean, `sha256sum * > SHA256SUMS` and cross.

## 3. Inside, with a registry

```sh
docker load -i images.tar
docker push harbor.internal/cairn:1.12.0
docker push harbor.internal/gatus:5.36.0
```

Nothing cairn-specific happens here: it is ordinary registry work, with whatever
Harbor, Zot or `registry:2` you already run. The chart wants one value.

```yaml
image:
  repository: harbor.internal/cairn
  tag: "1.12.0"
```

The charts belong in there too, if your registry takes them. Both are ordinary
OCI artifacts, so a project named `helm` holds them next to the images and
nothing else has to be installed:

```sh
helm push cairn-1.12.0.tgz oci://harbor.internal/helm
helm push gatus-1.5.0.tgz oci://harbor.internal/helm
```

```console
$ curl -s https://harbor.internal/v2/_catalog
{"repositories":["helm/cairn","helm/gatus"]}
```

From then on the cluster installs from `oci://harbor.internal/helm/cairn`, and
the only files you keep are your values. That is also what makes the whole thing
consumable by Argo CD: see
[Helm through Argo CD](helm.md#from-a-registry-through-argo-cd).

## 4. Inside, without a registry

Every node needs the images in its own store, and a cluster stopped speaking
Docker years ago:

```sh
sudo ctr -n k8s.io images import images.tar          # containerd
sudo k3s ctr images import images.tar                # k3s
sudo nerdctl --namespace k8s.io load -i images.tar   # nerdctl
```

The namespace matters more than it looks. An image imported outside `k8s.io` is
invisible to the kubelet, which then tries to pull it and fails on a hostname
that does not resolve.

Nothing else changes: the chart already ships `imagePullPolicy: IfNotPresent`,
so an image that is there is used and no pull is attempted. The events tell you
it worked:

```console
$ kubectl get events --field-selector reason=Pulled
Container image "harbor.internal/cairn:1.12.0" already present on machine
Container image "harbor.internal/gatus:5.36.0" already present on machine
```

## 5. Install

The icons become a ConfigMap, and where you mount it matters: cairn looks for
`<assets>/icons/<slug>.svg`, so the mount point is `/assets/icons`, not
`/assets`.

```sh
kubectl create configmap cairn-icons --from-file=assets/icons/
```

```yaml
# values.yaml
image:
  repository: harbor.internal/cairn
  tag: "1.12.0"

config:
  site.yaml: |
    title: Our tools
    locales: [fr, en]
    status:
      gatus: http://gatus       # the Service below, port 80, not 8080
      linked: false             # visitors have no route to Gatus
  services.yaml: |
    - id: pad
      url: https://pad.internal
      icon: hedgedoc
      name: { fr: Bloc-notes, en: Notepad }
      desc: { fr: Écrire à plusieurs., en: Write together. }

extraVolumes:
  - name: icons
    configMap:
      name: cairn-icons
extraVolumeMounts:
  - name: icons
    mountPath: /assets/icons
    readOnly: true

ingress:
  enabled: true
  className: nginx
  host: tools.internal
```

If the charts went into your registry at step 3, install from there rather than
from the tarballs. Same artifact, but the cluster now has one place to get it
and one version to name, which is also what Argo CD will read:

```sh
helm registry login harbor.internal --ca-file /etc/pki/internal-ca.crt
helm install cairn oci://harbor.internal/helm/cairn --version 1.12.0 \
  --ca-file /etc/pki/internal-ca.crt -f values.yaml
```

The install URL carries the chart name, the push URL did not: you pushed to
`oci://harbor.internal/helm` and install from `oci://harbor.internal/helm/cairn`.
`helm` works the last segment out of the archive itself.

Two flags you will need against an internal registry. `--ca-file` if it serves
your own certificate, on both the login and the install, since a Harbor behind a
private PKI fails exactly like everything else on this page. `--plain-http` if
it serves no TLS at all. And note that `helm registry login` writes its own
credential store: a `docker login` already done on that host does not count.

Without a registry, install what you carried:

```sh
helm install cairn ./cairn-1.12.0.tgz -f values.yaml
```

`linked: false` is a decision rather than an omission. Without it the pills link
visitors to `http://gatus`, an address their browser will never resolve.

Gatus installs from its own chart, out of the tarball you carried or out of your
registry. No manifest to hand-write, and one less thing that drifts:

```yaml
# gatus-values.yaml
image:
  repository: harbor.internal/gatus   # ghcr.io/twin/gatus with a route out
  tag: "5.36.0"                       # the chart's own default trails the app

config:
  storage: { type: memory }
  # the endpoints cairn -emit-gatus wrote for you
  endpoints:
    - name: pad
      url: https://pad.internal
      interval: 5m
      conditions: ["[STATUS] == 200"]
```

```sh
helm install gatus oci://harbor.internal/helm/gatus --version 1.5.0 \
  -f gatus-values.yaml                       # or ./gatus-1.5.0.tgz without a registry
```

Two details decide whether the pills ever turn green, and both are easy to get
wrong from memory.

**The release name becomes the Service name.** Called `gatus`, the release gives
you a Service called `gatus`; called anything else, you get `<release>-gatus`,
and cairn's `status.gatus` has to say so.

**That chart's Service listens on 80, not 8080.** Its container does listen on
8080 and the Service forwards to it, so the address cairn polls is
`http://gatus`, not `http://gatus:8080`. A wrong port here fails the way this
whole page fails: nothing crashes, the site serves, and every pill stays grey.

And one that matters only here: **the chart ships an example endpoint pointing
at `https://example.org`**, probed every 60 seconds. Your own `config.endpoints`
replaces it, because Helm replaces lists rather than merging them, but a values
file that only sets `config.storage` leaves it in place. An isolated network
then spends its life watching a pod fail to reach the internet once a minute.

### Exposing Gatus, or not

The values above keep Gatus to itself: cairn polls it from pod to pod, no
visitor ever reaches it, and `linked: false` states as much on the cairn side.
One less thing exposed, and the default here.

If you do want the status page reachable, two settings have to agree. Turn on
the chart's Ingress:

```yaml
ingress:
  enabled: true
  ingressClassName: nginx
  hosts:
    - status.internal
  tls:
    - secretName: gatus-tls
      hosts:
        - status.internal
```

and on the cairn side, replace `linked: false` with the address a visitor should
land on:

```yaml
status:
  gatus: http://gatus              # what the server polls, pod to pod
  page: https://status.internal    # where the pill sends a visitor
```

Enable one without the other and you get either a status page no pill points
to, or pills aimed at a host no browser can resolve.

The two charts do not spell their Ingress the same way, which catches everyone
copying from one file into the other:

|       | cairn chart                        | Gatus chart                             |
| ----- | ---------------------------------- | --------------------------------------- |
| class | `className`                        | `ingressClassName`                      |
| host  | `host:`, one string                | `hosts:`, a list                        |
| TLS   | `tls.enabled` and `tls.secretName` | `tls:`, a list of `{secretName, hosts}` |

cairn takes one host and one path because that is the shape of every
self-hosted cairn. The Gatus chart covers the general case.

## 6. Self-signed certificates

Sooner or later a probe meets a certificate nobody outside your network has ever
heard of. Two different things can fail here, and they fail differently. Read
the symptom before touching anything:

| What the page shows             | What cannot verify what                    |
| ------------------------------- | ------------------------------------------ |
| Pills read **Offline**, in red  | Gatus cannot verify the services it probes |
| Pills read **Unknown**, neutral | cairn cannot verify Gatus                  |

The logs name it either way:

```console
# Gatus, per endpoint, in its API and its log
"errors":["Get \"https://pad.internal\": tls: failed to verify certificate:
           x509: certificate signed by unknown authority"]

# cairn, once per poll
status: Get "https://status.internal/api/v1/endpoints/statuses": tls: failed to
        verify certificate: x509: certificate signed by unknown authority
        (dots hidden until gatus answers)
```

The distinction is the useful part. A red pill means Gatus reached its verdict
and the verdict is bad. "Unknown" means cairn has no verdict to show, so it
declines to invent one; the log line that follows says why, and its
parenthesis, "dots hidden until gatus answers", is cairn telling you the state
it is refusing to guess. Two broken links, two different faces, which matters on
the day both are broken at once.

### One bundle, however many certificates

A PEM file holds as many certificates as you concatenate into it, and both
programs read all of them. One file, one ConfigMap, both charts:

```sh
cat pad.crt photos.crt wiki.crt > ca-bundle.crt
kubectl create configmap ca-bundle --from-file=ca-certificates.crt=ca-bundle.crt
```

Two things worth knowing before you reissue anything:

**A self-signed server certificate works as a trust anchor even when it carries
`CA:FALSE`**, which is what most generators and cert-manager's `selfSigned`
issuer produce. There is nothing to regenerate.

**The bundle replaces the public store, it does not extend it.** Anything
presenting a certificate from a public authority stops verifying the moment you
mount your own. If you have both kinds, concatenate both:

```sh
cat /etc/ssl/certs/ca-certificates.crt mine.crt > ca-bundle.crt
```

### Gatus

Gatus has no setting for a custom authority: its `tls:` block configures client
certificates, not verification. It is a Go binary, so the environment variable
does the work, and the chart takes both halves:

```yaml
env:
  SSL_CERT_FILE: /etc/ssl/internal/ca-certificates.crt
extraVolumeMounts:
  - name: ca-bundle
    mountPath: /etc/ssl/internal
    readOnly: true
    existingConfigMap: ca-bundle
```

That chart carries the volume source inside the mount entry, which is why there
is an `existingConfigMap` here and no separate `extraVolumes` list.

If one target resists, `client.insecure: true` can be set on that endpoint alone
rather than on the whole instance. Keep it as a last resort: it also hides the
day that certificate changes.

### cairn

Ask first whether there is any TLS to verify. Between pods, `http://gatus`
reaches the Service directly, never leaves the cluster, and meets no certificate
at all. That is the default on this page, and it makes this half of the section
unnecessary.

If cairn does poll over `https://`, the chart exposes no environment values, so
the bundle replaces the one the image ships:

```yaml
extraVolumes:
  - name: ca
    configMap:
      name: ca-bundle          # the same one Gatus reads
extraVolumeMounts:
  - name: ca
    mountPath: /etc/ssl/certs/ca-certificates.crt
    subPath: ca-certificates.crt
    readOnly: true
```

`subPath` replaces that single file and leaves the rest of the image alone,
which matters more here than elsewhere: `FROM scratch` means there is no shell
to repair anything in place afterwards.

### Adding a certificate later means a restart

Both programs read their trust store once, at startup. Updating the ConfigMap
propagates the new file and nothing happens: the certificate stays untrusted
until the pods restart.

```sh
kubectl rollout restart deploy/gatus deploy/cairn
```

This is the one place where cairn does not behave like the rest of this
deployment. Its configuration reloads by itself within the minute; its trust
store does not.

### Proving it, rather than hoping

Any "it works now" that follows a TLS change deserves a negative control. Keep
one endpoint whose certificate is deliberately absent from the bundle, and check
that it still fails:

```plain
alpha    success=true     in the bundle
beta     success=true     in the bundle, and CA:FALSE
gamma    success=false    absent: x509: certificate signed by unknown authority
```

If everything turns green including that last one, verification is switched off
somewhere rather than fixed, and you have traded a visible failure for an
invisible one.

## 7. Checking it without the internet

```sh
kubectl port-forward svc/cairn 8080:80
curl -s localhost:8080/readyz                                                  # ready
curl -s localhost:8080/fr/ | grep -c 'jsdelivr\|cdn\.'                         # 0
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/assets/icons/nginx.svg  # 200
```

The middle one belongs in your runbook. A page that answers anything but `0`
still depends on the outside, and it will look perfectly fine from a workstation
that happens to have a route.

`kubectl exec` will not help you here: the image is `FROM scratch`, so there is
no shell, no busybox, nothing to attach to. What you need is in the logs, on
`/readyz`, and in what the page returns.

## 8. Updating

The same crossing, then:

```sh
helm upgrade cairn oci://harbor.internal/helm/cairn --version 1.13.0 -f values.yaml
```

Always pass your full values file. As soon as one `-f` is present, Helm stops
reusing the previous release's values, and anything you leave out falls back to
the chart's defaults.

A config edit alone does not even need that: change the ConfigMap and cairn
picks it up within the minute, no pod restarted, nothing to reload by hand.

---

Everything on this page was run end to end on a single-node cluster before it
was written: both charts installed from local `.tgz` files, images imported into
the node under a hostname that resolves nowhere, icons served from a ConfigMap,
Gatus reached over its Service on 80, and the pills settling green and red.

The certificate section was built the same way, against three hosts with three
different self-signed certificates: two in the bundle, one deliberately left
out, so that the fix could be told apart from a fix that merely stops checking.
`CA:FALSE` trusting correctly, and the restart being needed after a bundle
update, are both measured rather than reasoned.

The registry push in step 3 is the one step taken on faith, since that part is
your Harbor and not ours.

Next: [Reverse proxies](reverse-proxies.md)
