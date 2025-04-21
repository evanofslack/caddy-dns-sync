# Caddy DNS Synchronizer

Automatically synchronize reverse-proxy configurations from Caddy server with Cloudflare DNS records.

## Development

### Prerequisites

- Docker and Docker Compose
- Cloudflare API token with DNS read/edit permissions

### Quick Start

1. **Set up environment variables**:

   Create a `.env` file in the project root:

   ```
   CLOUDFLARE_API_TOKEN=your_cloudflare_api_token
   ```

2. **Start the development environment**:

   ```bash
   docker-compose -f dev/docker-compose.yaml up --build
   ```

3. **Verify services are running**:

   Check Caddy's admin API:

   ```bash
   curl http://localhost:2019/config/
   ```

   The sync service logs should show polling activity and dry-run operations.

### Testing DNS Synchronization

1. **Add a test domain to Caddy**:

   ```bash
   curl -X POST http://localhost:2019/load \
     -H "Content-Type: text/caddyfile" \
     -d 'newdomain.example.com { reverse_proxy localhost:8080 }'
   ```

2. **Watch the sync service logs**:

   You should see the service detect the new domain and perform a dry-run of DNS operations.

3. **Enable live mode**:

   To apply actual DNS changes, edit `dev/config.yaml` and set `dryRun: false`, then restart the service.

### Cleanup

```bash
docker-compose -f dev/docker-compose.yaml down -v
```

## Configuration

See `dev/config.yaml` for configuration options. Key settings:

- `syncInterval`: How often to check for changes
- `dns.zones`: DNS zones to manage
- `reconcile.protectedRecords`: Records that will never be modified
