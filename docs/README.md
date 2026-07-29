# cairn documentation

Start here, every page is one click away. To see cairn running before
reading anything: <https://cairn.libresoftware.cloud>.

**First steps**

- [Getting started](getting-started.md): zero to a served page in five minutes.

**Configuration**

- [Services](configuration/services.md): the cards: every key, defaults, examples.
- [Site](configuration/site.md): title, welcome note, header links, footer, hosted legal pages.
- [Writing text](configuration/text.md): paragraphs and the markdown subset for long fields.
- [Theming](configuration/theming.md): accent color, dark mode, `custom.css`.
- [Languages](configuration/i18n.md): locales, fallbacks, UI string overrides.

**Deployment**

- [Docker Compose](deployment/docker-compose.md): the hardened reference setup.
- [Podman](deployment/podman.md): the same container as a systemd quadlet.
- [Bare binary](deployment/binary.md): no container, a hardened systemd unit.
- [Kubernetes](deployment/kubernetes.md): a ConfigMap, a Deployment, no volume.
- [Helm](deployment/helm.md): the same four objects, and your Ingress from values.
- [Air-gapped](deployment/airgap.md): what crosses the white station, and the icons that catch everyone.
- [Reverse proxies](deployment/reverse-proxies.md): Caddy, Traefik, Nginx, Pangolin.

**Recipes**

- [Icons](recipes/icons.md): dashboard-icons slugs, selfh.st, your own files.
- [Multiple files](recipes/multiple-files.md): one YAML per category.
- [Status page](recipes/gatus.md): pairing cairn with Gatus.
- [Coming from Homer or Homepage](recipes/migration.md): your config maps over.

**Reference**

- [Configuration reference](reference.md): every key, flag and endpoint on one page.
- [FAQ](faq.md): tracking, JavaScript, auth, cookies, breakage.
- [Comparison](comparison.md): when Homer or Homepage fit better.
