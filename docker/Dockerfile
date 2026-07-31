# Pinned by digest, like every action in the workflows: the tag alone lets the
# compiler that produces a released binary change under us between two builds
# of the same commit. Dependabot watches /docker and moves it.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
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
