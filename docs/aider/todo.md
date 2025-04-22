# Development Roadmap

## High Priority

- [x] Add docker-compose.yaml for development testing
- [x] Implement zone configuration from DNS settings
- [x] Add zone validation in reconciliation
- [x] Support nested subdomains in zone matching
- [ ] Add retry logic with exponential backoff
- [ ] Create Prometheus metrics endpoint
- [ ] Add Caddy API authentication support

## Medium Priority

- [ ] Support multiple DNS providers (AWS Route53, DigitalOcean)
- [ ] Implement state schema versioning
- [ ] Add health check endpoints
- [ ] Configuration hot-reload support

## Future Features

- [ ] Webhook-based config updates
- [ ] TLS certificate validation
- [ ] Distributed locking mechanism

## Metrics Plan

### Synchronization Process
- `sync_runs_total{status="success|failure",dry_run="true|false"}` - Counter
- `sync_duration_seconds` - Histogram (bucket by sync time)

### DNS Operations
- `dns_changes_total{operation="create|update|delete",zone,record_type="A|CNAME",status="success|failure"}` - Counter
- `dns_provider_errors_total{error_type="api|network|validation"}` - Counter

### State Management
- `state_changes_total{operation="save|load",status="success|failure"}` - Counter
- `state_size_bytes` - Gauge (serialized state size)

### Caddy Interactions
- `caddy_config_fetches_total{status="success|failure"}` - Counter
- `caddy_domains_discovered` - Gauge (number of configured domains)

### Resource Tracking
- `pending_changes` - Gauge (changes awaiting processing)
- `reconciliation_latency_seconds` - Histogram (time from config change to DNS update)
