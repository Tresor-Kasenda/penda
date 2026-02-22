# Penda Documentation Index

This is the central entry point for the current `penda` documentation.

## What Is Implemented

Current implementation covers:
- Core app/kernel, routing, params, wildcard routes
- Middleware pipeline (global, group, route-level)
- Built-in middleware (recovery, logger, request ID, timeout, CORS, security headers, rate limit, CSRF)
- Request context helpers (query, headers, locals, response helpers)
- Request parsing/binding (JSON, form, multipart)
- Centralized error handling (`OnError`, `OnStatus`, typed HTTP errors)
- Templates and static file serving
- Config loading (defaults, profiles, JSON/YAML/TOML files, env precedence)
- Blueprints/modules (including local templates/static mounts)
- CLI commands (`new`, `run`, `routes`, `doctor`)
- Testing toolkit (`framework/testing`, JSON/cookies/multipart helpers)
- Observability (`/metrics`, health/readiness helpers)
- ORM integration (GORM-based, multi-SGBD, custom dialector registry)

## Recommended Reading Order

1. `docs/premiere-partie.md`
2. `docs/advanced-phases.md`
3. `docs/orm.md`
4. `docs/examples.md`
5. `docs/api-reference.md`

## Documentation Map

### Getting Started

- `docs/premiere-partie.md`
  - installation flow (Okapi-style structure)
  - first app
  - core concepts (App, Context, routing)

### Advanced Features (Phases 6-12)

- `docs/advanced-phases.md`
  - error handling
  - templates/static
  - config integration
  - blueprints
  - CLI
  - testing toolkit
  - observability/security

### ORM / Database Integration

- `docs/orm.md`
  - GORM integration
  - built-in SQL dialectors
  - custom dialector registration (support for other SQL databases)
  - request-scoped DB injection

### Examples

- `docs/examples.md`
  - `hello`
  - `rest-api` (ORM CRUD)
  - `web-app`

### Full API Reference

- `docs/api-reference.md`
  - package-by-package API surface
  - behavior notes and caveats
  - common usage patterns

### Project Roadmap

- `ROADMAP_MICRO_FRAMEWORK_GO.md`
  - original implementation roadmap
  - learning progression

### Release / Maintenance

- `CHANGELOG.md`
- `RELEASE.md`
- `MIGRATION.md`

## Quick Links

Run the demo app:

```bash
make run
```

List demo routes:

```bash
go run ./cmd/penda routes
```

Run tests:

```bash
make test
```
