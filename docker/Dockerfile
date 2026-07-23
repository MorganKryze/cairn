FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY src ./src
RUN go vet ./... && go test ./...
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /cairn ./src

FROM scratch
COPY --from=build /cairn /cairn
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/cairn"]
