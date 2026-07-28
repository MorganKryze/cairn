FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
# The scratch image ships no CA bundle, so cairn cannot verify TLS on its own:
# polling a public https Gatus fails with x509: unknown authority. Copy the
# bundle in (below); install it here so the build never depends on the base
# image happening to carry it.
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
