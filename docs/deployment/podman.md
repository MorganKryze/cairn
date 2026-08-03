# Podman

[Podman](https://podman.io) runs the same OCI containers as Docker with two
differences self-hosters tend to like: there is no daemon (each container is
a plain process, no root service in the background), and it is designed to
run rootless. If you have Podman, `podman run` accepts everything the
[compose page](docker-compose.md) shows.

The idiomatic way, though, is a **quadlet**: you describe the container in a
small unit file and systemd owns it like any other service: starts it at
boot, restarts it, logs to journald.

## The quadlet

Save as `~/.config/containers/systemd/cairn.container` (rootless) or
`/etc/containers/systemd/cairn.container` (root):

```ini
[Unit]
Description=cairn directory page

[Container]
Image=morgankryze/cairn:stable
PublishPort=8080:8080
Volume=%h/cairn/config:/config:ro,Z
ReadOnly=true
DropCapability=all
NoNewPrivileges=true
HealthCmd=/cairn -healthcheck
HealthInterval=30s
HealthStartPeriod=5s
AutoUpdate=registry

[Service]
Restart=on-failure

[Install]
WantedBy=default.target
```

The same hardening as the compose file, in quadlet vocabulary. The `,Z` on
the volume relabels it for SELinux (Fedora family); harmless elsewhere.
`%h` is your home directory.

## Run it

```sh
systemctl --user daemon-reload
systemctl --user start cairn
```

Quadlet units are not `systemctl enable`d; the `[Install]` section already
starts it at boot. For a rootless unit to start without you logging in:

```sh
loginctl enable-linger $USER
```

## Updates

`AutoUpdate=registry` lets Podman refresh the image and restart the unit
when the tag moves:

```sh
systemctl --user enable --now podman-auto-update.timer
```

With the `stable` tag above, that means every cairn release, automatically,
tests already passed. Pin a [semver tag](docker-compose.md#updating) instead
if you prefer to move deliberately.

Next: [Bare binary](binary.md)
