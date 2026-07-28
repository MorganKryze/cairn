# Bare binary

cairn is one static binary: no container needed if a plain process suits
you better. Every [release](https://github.com/MorganKryze/cairn/releases)
attaches builds for Linux (amd64, arm64) and macOS (arm64), or build your
own with Go:

```sh
git clone https://github.com/MorganKryze/cairn.git && cd cairn
go build -o cairn ./src
```

Run it against a config folder; every flag is in the
[reference](../reference.md#endpoints):

```sh
./cairn -config ./config -assets ./assets -addr :8080
```

## As a systemd service

```ini
# /etc/systemd/system/cairn.service
[Unit]
Description=cairn, the directory page for the people you host for
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/cairn -config /etc/cairn/config -assets /etc/cairn/assets
DynamicUser=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```sh
install -m 755 cairn /usr/local/bin/cairn
mkdir -p /etc/cairn/config
cairn -init > /etc/cairn/config/services.yaml
systemctl enable --now cairn
```

The same hardening story as the container: `DynamicUser` runs it as a
throwaway unprivileged user, `ProtectSystem=strict` makes the filesystem
read-only, and cairn never writes anything anyway. Edit the YAML and the
page follows within seconds; `journalctl -u cairn -f` shows the same logs
`docker logs` would.

Next: [Kubernetes](kubernetes.md)
