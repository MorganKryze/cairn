FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY src ./src
COPY schema ./schema
RUN go vet ./... && go test ./...
ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /cairn ./src

FROM scratch
COPY --from=build /cairn /cairn
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/cairn"]
