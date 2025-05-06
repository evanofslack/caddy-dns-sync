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
    - CADDY_DNS_SYNC_CADDY_URL=http://caddy:2019 # caddy admin endpoint
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

## Metrics

exposes prometheus metrics at `/metrics`

```
# HELP caddy_dns_sync_badgerdb_requests_total Total badgerdb requests
# TYPE caddy_dns_sync_badgerdb_requests_total counter
caddy_dns_sync_badgerdb_requests_total{operation="read",status="success"} 5
# HELP caddy_dns_sync_caddy_entries_current Current known caddy entries
# TYPE caddy_dns_sync_caddy_entries_current gauge
caddy_dns_sync_caddy_entries_current{reverse_proxy="true"} 261
# HELP caddy_dns_sync_caddy_requests_total Total caddy requests
# TYPE caddy_dns_sync_caddy_requests_total counter
caddy_dns_sync_caddy_requests_total{code="200",status="success"} 5
# HELP caddy_dns_sync_dns_operations_total Total DNS operations managed by app
# TYPE caddy_dns_sync_dns_operations_total counter
caddy_dns_sync_dns_operations_total{operation="create",type="A",zone="eslack.net"} 435
caddy_dns_sync_dns_operations_total{operation="create",type="TXT",zone="eslack.net"} 435
# HELP caddy_dns_sync_dns_requests_total Total DNS provider requests
# TYPE caddy_dns_sync_dns_requests_total counter
caddy_dns_sync_dns_requests_total{operation="read",status="success",zone="eslack.net"} 5
# HELP caddy_dns_sync_sync_duration_milliseconds Duration of synchronization runs in milliseconds
# TYPE caddy_dns_sync_sync_duration_milliseconds histogram
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.005"} 0
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.01"} 0
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.025"} 0
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.05"} 0
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.1"} 0
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.25"} 1
caddy_dns_sync_sync_duration_milliseconds_bucket{le="0.5"} 2
caddy_dns_sync_sync_duration_milliseconds_bucket{le="1"} 5
caddy_dns_sync_sync_duration_milliseconds_bucket{le="2.5"} 5
caddy_dns_sync_sync_duration_milliseconds_bucket{le="5"} 5
caddy_dns_sync_sync_duration_milliseconds_bucket{le="10"} 5
caddy_dns_sync_sync_duration_milliseconds_bucket{le="+Inf"} 5
caddy_dns_sync_sync_duration_milliseconds_sum 2.6130641150000002
caddy_dns_sync_sync_duration_milliseconds_count 5
# HELP caddy_dns_sync_sync_runs_total Total number of synchronization runs
# TYPE caddy_dns_sync_sync_runs_total counter
caddy_dns_sync_sync_runs_total{status="success"} 5

```
