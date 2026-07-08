FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /rankflow-api ./cmd/api
RUN cat <<'EOF' > /tmp/healthcheck.go
package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
EOF
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /rankflow-api-healthcheck /tmp/healthcheck.go

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /rankflow-api /usr/local/bin/rankflow-api
COPY --from=builder /rankflow-api-healthcheck /usr/local/bin/rankflow-api-healthcheck
COPY config.yaml /app/config.yaml

EXPOSE 8080

CMD ["/usr/local/bin/rankflow-api"]
