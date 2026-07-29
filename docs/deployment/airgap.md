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

| Artifact | Size | Where it comes from |
| --- | --- | --- |
| `cairn-1.12.0.tgz` | 4 KB | `helm pull`, the signed artifact rather than the folder in git |
| `gatus-1.5.0.tgz` | 9 KB | the Gatus project's own chart |
| `images.tar` | 26 MB | `docker save` of cairn and Gatus |
| `assets/icons/*.svg` | a few KB | what `cairn -emit-icons` downloads |
| your values | a few KB | one file per chart, and they are yours to keep |

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
helm install gatus ./gatus-1.5.0.tgz -f gatus-values.yaml
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

## 6. When Gatus is behind your own CA

The image carries the public CA bundle, which is what lets cairn poll a public
Gatus over TLS. Behind a private PKI it knows none of your certificates: the
pills stay grey and the log shows `x509: certificate signed by unknown
authority`. There is no shell in the image to fix that in place, so mount your
bundle over the one it ships:

```yaml
extraVolumes:
  - name: ca
    configMap:
      name: internal-ca      # kubectl create configmap internal-ca --from-file=ca-certificates.crt=ca.crt
extraVolumeMounts:
  - name: ca
    mountPath: /etc/ssl/certs/ca-certificates.crt
    subPath: ca-certificates.crt
    readOnly: true
```

The `subPath` replaces that one file and leaves the rest of the image alone.
Plain `http://` between pods needs none of this.

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
helm upgrade cairn ./cairn-1.13.0.tgz -f values.yaml
```

Always pass your full values file. As soon as one `-f` is present, Helm stops
reusing the previous release's values, and anything you leave out falls back to
the chart's defaults.

A config edit alone does not even need that: change the ConfigMap and cairn
picks it up within the minute, no pod restarted, nothing to reload by hand.

---

Both routes above were run end to end on a single-node cluster before this page
was written: chart installed from a local `.tgz`, images imported into the node
under a hostname that resolves nowhere, icons served from a ConfigMap, the CA
bundle replaced through a `subPath`, and the pills settling green and red. The
registry push in step 3 is the one step taken on faith, since that part is your
Harbor and not ours.

Next: [Reverse proxies](reverse-proxies.md)
