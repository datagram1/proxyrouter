# Build stage
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/proxyrouter ./cmd/proxyrouter

# Runtime stage
FROM gcr.io/distroless/base-debian12
COPY --from=build /out/proxyrouter /usr/local/bin/proxyrouter
COPY configs/config.yaml /etc/proxyrouter/config.yaml
VOLUME ["/var/lib/proxyr"]
EXPOSE 8080 1080 8081
ENTRYPOINT ["/usr/local/bin/proxyrouter", "-config", "/etc/proxyrouter/config.yaml"]
