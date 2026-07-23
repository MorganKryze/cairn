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

## Scope worth knowing

cairn's attack surface is deliberately small: no auth, no database, no
outbound requests except the one Gatus URL you configure, config mounted
read-only, `FROM scratch` with no shell. Every response carries a strict
`Content-Security-Policy` (inline fragments allowed by hash only) plus the
standard hardening headers, and CI runs `govulncheck` and a Trivy image
scan weekly. Reports about the visitor-facing
page (XSS through config values, header handling, cache poisoning) are the
interesting ones.
