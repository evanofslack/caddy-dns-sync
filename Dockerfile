FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates git

# environment variables for Go modules and DNS
ENV GOPROXY=https://proxy.golang.org,direct
ENV GO111MODULE=on
ENV GODEBUG=netdns=go

COPY go.mod go.sum* ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o caddy-dns-sync .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# non-root user
RUN addgroup -g 1000 appuser && \
    adduser -u 1000 -G appuser -s /bin/sh -D appuser

# directories for state data
RUN mkdir -p /data && \
    chown -R appuser:appuser /data

WORKDIR /app

COPY --from=builder /app/caddy-dns-sync .

# ownership and permissions
RUN chown -R appuser:appuser /app && \
    chmod +x /app/caddy-dns-sync

USER appuser

VOLUME ["/data"]

CMD ["/app/caddy-dns-sync"]
