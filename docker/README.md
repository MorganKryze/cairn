# cairn

The directory page for the people you host services for: no account,
multilingual, live status, one tiny container.

![The cairn home page: a welcome note, service cards grouped by category with live status pills, and a category trail in the margin](https://raw.githubusercontent.com/MorganKryze/cairn/main/docs/assets/home-light.png)

You run things for other people. Family, a club, a team, a floor of an office.
They have no idea what any of it is called or where it lives, and they should
not have to. cairn is the one page you send them: what you host, what it is
for, whether it is up, in their own language.

You write two YAML files. cairn serves them. There is no database, no account
system, no admin panel, and nothing to log into.

**[Live demo](https://cairn.libresoftware.cloud)** ·
**[Source and documentation](https://github.com/MorganKryze/cairn)**

## Run it

```sh
docker run --rm -p 8080:8080 -v ./config:/config:ro morgankryze/cairn:stable
```

Or with Compose, which is how most people run it:

```yaml
services:
  cairn:
    image: morgankryze/cairn:stable
    ports:
      - 8080:8080
    volumes:
      - ./config:/config:ro
    read_only: true
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
```

The [getting started guide](https://github.com/MorganKryze/cairn/blob/main/docs/getting-started.md)
has the two config files to put in `./config`, and it is a five minute read.

## Tags

| Tag                                                     | Points at                          |
| ------------------------------------------------------- | ---------------------------------- |
| `stable`, `latest`                                      | the newest release                 |
| `<major>`, `<major>.<minor>`, `<major>.<minor>.<patch>` | that major, minor or exact version |
| `unstable`                                              | every commit on the main branch    |
| a commit hash                                           | that exact build                   |

Pin the exact version if something else depends on the page looking the way it
looks today. `1` and `stable` move on every release; the
[upgrading notes](https://github.com/MorganKryze/cairn/blob/main/docs/upgrading.md)
say what a version can refuse to load.

Built for `linux/amd64` and `linux/arm64`, so a Raspberry Pi is a first-class
target rather than an afterthought.

## What is in it

A single static Go binary on `scratch`, with a CA bundle and nothing else. No
shell, no package manager, no interpreter: there is nothing in the image to
run but cairn. It drops every capability, runs as a non-root user and needs no
writable filesystem, which is what makes the Compose block above possible.

## Verify what you pulled

This image is the same object published to
[GitHub Container Registry](https://github.com/MorganKryze/cairn/pkgs/container/cairn),
copied here digest for digest rather than built twice. It is signed keylessly,
so there is no maintainer key to steal, and it carries SLSA build provenance
and a software bill of materials.

```sh
cosign verify morgankryze/cairn:stable \
  --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The [security policy](https://github.com/MorganKryze/cairn/blob/main/SECURITY.md)
explains what that proves and how to report a problem.

## Licence

GPL-3.0. Issues, translations and pull requests are welcome on
[GitHub](https://github.com/MorganKryze/cairn).
