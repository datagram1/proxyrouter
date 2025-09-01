# Build stage
FROM golang:1.23 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/proxyrouter-amd64 ./cmd/proxyrouter
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /out/proxyrouter-arm64 ./cmd/proxyrouter

# Runtime stage
FROM gcr.io/distroless/base-debian12
ARG TARGETARCH
COPY --from=build /out/proxyrouter-${TARGETARCH} /usr/local/bin/proxyrouter
COPY builds/docker/docker-config.yaml /etc/proxyrouter/config.yaml
COPY migrations /etc/migrations
VOLUME ["/var/lib/proxyr"]
EXPOSE 8080 1080 8081 6000
ENTRYPOINT ["/usr/local/bin/proxyrouter", "-config", "/etc/proxyrouter/config.yaml"]
