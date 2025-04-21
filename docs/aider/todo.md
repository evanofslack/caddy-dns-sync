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
