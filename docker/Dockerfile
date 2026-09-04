# Pinned by digest, like every action in the workflows: the tag alone lets the
# compiler that produces a released binary change under us between two builds
# of the same commit.
#
# Dependabot moves this pin, tag and digest both, after the seven-day cooldown
# in .github/dependabot.yml, and automerge.yml lands it once the required
# checks pass. It has done both: #110 took the tag from 1.26-alpine to
# 1.27-alpine, #112 the digest underneath it. There is no update-type filter,
# on purpose: a bump's size does not predict its safety, and the checks are the
# gate either way.
#
# Left alone this pin cost a release once. On 2026-08-17 it still held go1.26.5
# against a published 1.26.6: eight HIGH stdlib CVEs in the shipped binary, one
# of them an html/template XSS, which is the package every cairn page is
# rendered through. v1.19.3 was that and nothing else. The weekly Trivy scan
# stays the net for the gap between two proposals, since it reads the binary
# rather than this line.
#
# Read through mirror.gcr.io rather than straight from Docker Hub, for the same
# reason demo/compose.yaml is: an anonymous Hub pull shares one rate limit with
# every other GitHub-hosted runner on the planet, and it does not fail by
# saying so. It times out. That is what took down the image job of the build on
# main after 1.13.2 shipped. The mirror serves the same object: a digest is
# what the content hashes to, so the registry in front is only the road taken
# to reach it. This comment names no digest of its own, since Dependabot
# rewrites the one below and a copy up here went stale the first time it did.
FROM --platform=$BUILDPLATFORM mirror.gcr.io/library/golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build
# The scratch image ships no CA bundle, so cairn cannot verify TLS on its own:
# polling a public https Gatus fails with x509: unknown authority. Copy the
# bundle in (below); install it here so the build never depends on the base
# image happening to carry it.
#
# Deliberately unversioned: Alpine's repositories carry only the current
# ca-certificates, so pinning one turns a rebuild into a failure rather than
# into a reproducible result. The bundle in a released image is the newest at
# the moment it was built, which for a trust store is the property you want.
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY src ./src
COPY schema ./schema
RUN go vet ./... && go test ./...
ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /cairn ./src/cmd/cairn

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /cairn /cairn
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/cairn"]
