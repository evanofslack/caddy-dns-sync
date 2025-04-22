# Development Roadmap

## High Priority

- [x] Add docker-compose.yaml for development testing
- [x] Implement zone configuration from DNS settings
- [x] Add zone validation in reconciliation
- [x] Support nested subdomains in zone matching
- [x] Create Prometheus metrics endpoint
- [ ] Add health check endpoints
- [ ] Add retry logic with exponential backoff
- [ ] Add Caddy API authentication support
- [ ] Add option to force delete/cleanup all created records

## Medium Priority

- [ ] Support multiple DNS providers (AWS Route53, DigitalOcean)
- [ ] Configuration hot-reload support

## Future Features

- [ ] Implement state schema versioning
- [ ] TLS certificate validation
- [ ] Distributed locking mechanism
