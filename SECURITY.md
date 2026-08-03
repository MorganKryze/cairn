# Security policy

## Supported versions

The latest release and the `unstable` image built from `main`. Older tags
stay pullable but are not patched.

## Reporting a vulnerability

Use GitHub's private reporting: **Security → Report a vulnerability** on
this repository. Please do not open a public issue for anything you believe
is exploitable.

You can expect an acknowledgement within a week. If the report is confirmed,
the fix lands in a patch release and the advisory is published once the
release is out.

## Verifying what you pulled

Every release is signed and carries the record of how it was built, so you can
check that an artifact really came from this repository's CI and not from
someone in between.

The image is signed keylessly with [cosign](https://github.com/sigstore/cosign):
there is no maintainer key to steal, the signature is bound to the workflow
identity.

```sh
cosign verify ghcr.io/morgankryze/cairn:stable \
  --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The same image is published to Docker Hub as `morgankryze/cairn`, and the
command works there unchanged: the release copies the digest ghcr has already
signed rather than building a second time, so both names resolve to the same
object and the signature covers the same bytes. Substituting one name for the
other in any command on this page is safe, including the provenance inspection
below.

The [Helm chart](docs/deployment/helm.md) is published the same way, as an OCI
artifact next to the image, and signed by the same workflow:

```sh
cosign verify ghcr.io/morgankryze/charts/cairn:1.12.0 \
  --certificate-identity-regexp '^https://github.com/MorganKryze/cairn/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

It also ships SLSA build provenance and a software bill of materials:

```sh
docker buildx imagetools inspect ghcr.io/morgankryze/cairn:stable \
  --format '{{ json .Provenance }}'
```

The release binaries carry their own attestation:

```sh
gh attestation verify cairn_1.8.0_linux_amd64.tar.gz --repo MorganKryze/cairn
```

A `checksums.txt` is attached to each release too, for the plain `sha256sum -c`
route.

## Scope worth knowing

cairn's attack surface is deliberately small: no auth, no database, no
outbound requests except the one Gatus URL you configure, config mounted
read-only, `FROM scratch` with no shell. Every response carries a strict
`Content-Security-Policy` (inline fragments allowed by hash only) plus the
standard hardening headers, and CI runs `govulncheck` and a Trivy image
scan weekly. Reports about the visitor-facing
page (XSS through config values, header handling, cache poisoning) are the
interesting ones.
