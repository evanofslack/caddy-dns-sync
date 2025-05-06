# caddy-dns-sync

Automatically synchronize reverse-proxy configurations from Caddy server with Cloudflare DNS records

## Getting Started

1. **Set up environment variables**:

Copy `.env.example` to `.env` and fill in cloudflare api token

2. **Start container**:

Can run from a prebuilt container: `evanofslack/caddy-dns-sync:latest`

```yaml
services:
  caddy-dns-sync:
  image: evanofslack/caddy-dns-sync
  container_name: caddy-dns-sync
  ports:
    - "8080:8080" # only exposes metrics
  environment:
    - CADDY_DNS_SYNC_CLOUDFLARE_TOKEN=${CLOUDFLARE_TOKEN}
    - CADDY_DNS_SYNC_CADDY_URL=<http://caddy:2019> # caddy adin endpoint
    - CADDY_DNS_SYNC_ZONES=domain.com,other.com # list of zones to sync
    - CADDY_DNS_SYNC_DRYRUN=false # only make dns requests if false
    - CADDY_DNS_SYNC_PROTECTED_RECORDS=protect.domain.com # list of domains not to sync
  volumes:
    - caddy_dns_sync_state:/data

volumes:
  caddy_dns_sync_state:
```

## Development

Can run caddy and caddy-dns-sync built from local code side by side

```bash
docker-compose -f dev/docker-compose.yaml up --build
```
