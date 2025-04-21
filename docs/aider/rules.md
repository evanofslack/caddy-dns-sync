# Project Development Rules

## Code Standards

- Go 1.22+ compatibility
- Prefer standard library over dependencies
- Interface-driven design patterns
- 80%+ test coverage for core components
- Zero-downtime friendly patterns
- Context-aware operations
- Minimal comments and concise coding language

## Contribution Process

2. Update documentation with changes
3. Add/update tests

## Safety Mechanisms

- Protected records cannot be modified
- Dry-run mode enabled by default
- State changes are atomic
- Configuration validation on startup
- Exponential backoff for API calls
- Zone validation before operations
- Authentication for Caddy API access
