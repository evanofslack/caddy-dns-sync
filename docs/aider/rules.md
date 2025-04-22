# Project Development Rules

## Code Standards

- Go 1.22+ compatibility
- Prefer standard library over dependencies
- Interface-driven design patterns
- 80%+ test coverage for core components
- Zero-downtime friendly patterns
- Context-aware operations
- Minimal comments and concise coding language
- Consistent metric naming and labeling

## Contribution Process

2. Update documentation with changes
3. Add/update tests
4. Ensure metrics coverage for new features

## Safety Mechanisms

- Protected records cannot be modified
- TXT record ownership verification for all managed records
- Dry-run mode enabled by default
- State changes are atomic
- Configuration validation on startup
- Exponential backoff for API calls
- Zone validation before operations
- Authentication for Caddy API access
- Low-cardinality metric labels

## Metrics Guidelines

- Follow Prometheus naming conventions (snake_case)
- Keep label cardinality low (avoid unbounded labels)
- Use appropriate metric types (counter, gauge, histogram)
- Include status labels for success/failure tracking
- Document all metrics in code and documentation
