# Pinned by digest, like every action in the workflows: the tag alone lets the
# compiler that produces a released binary change under us between two builds
# of the same commit.
#
# Nothing moves this pin automatically. Dependabot compares tags, and its log
# reads "Latest version is 1.26-alpine" every week: the tag is already the
# newest, so a digest sliding from one patch release to the next underneath it
# is invisible to it. Left alone the pin holds the Go it was written with, and
# on 2026-08-17 that was 1.26.5 against a released 1.26.6, eight HIGH stdlib
# CVEs in the shipped binary.
#
# What catches it is the weekly Trivy scan, which reads the binary rather than
# this line, and fires once the Go left behind carries a published CVE. That is
# late by design: raising the pin on every golang rebuild would be a weekly red
# build for nothing.
#
# Read through mirror.gcr.io rather than straight from Docker Hub, for the same
# reason demo/compose.yaml is: an anonymous Hub pull shares one rate limit with
# every other GitHub-hosted runner on the planet, and it does not fail by
# saying so. It times out. That is what took down the image job of the build on
# main after 1.13.2 shipped.
#
# The digest is the same object either way, verified rather than assumed: both
# registries answer sha256:3889b425… for this reference. A digest is what the
# content hashes to, so the mirror cannot serve a different image under it; the
# registry in front is only the road taken to reach it.
FROM --platform=$BUILDPLATFORM mirror.gcr.io/library/golang:1.26-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS build
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
