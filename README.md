# penda

`penda` is a learning-oriented micro-framework in Go, inspired by Flask.

## Current state

Implemented foundations:
- Project scaffold (`go.mod`, `Makefile`, basic command entrypoint)
- HTTP kernel (`App`, `Run`, `ServeHTTP`)
- Routing with HTTP method dispatch
- Path params (`:id`) and wildcard segments (`*path`)
- Middleware pipeline (global, group, route-level)
- Built-in middleware (`Recovery`, `Logger`, `RequestID`, `Timeout`, `CORS`, `SecurityHeaders`, `RateLimit`, `CSRF`)
- Request context (`Context`) with params and local values
- Request parsing/binding (`BindJSON`, `BindForm`, multipart helpers)
- Response helpers (`Status`, `Text`, `JSON`, `HTML`, `Redirect`, `File`, `Download`)
- Header/cookie helpers
- Centralized error handling (`OnError`, `OnStatus`, `HTTPError`)
- Templates and static serving (`LoadTemplates`, `Render`, `Static`)
- Config package (`framework/config`) with profiles + JSON/YAML/TOML file/env resolution
- ORM integration (`framework/orm`) with GORM + custom dialector registry
- Blueprints/modules (`framework/blueprint`) with local templates/static mounts
- CLI commands (`penda new`, `penda run`, `penda routes`, `penda doctor`)
- Testing toolkit (`framework/testing`) with JSON/cookie assertions and multipart helpers
- Observability package (`/metrics`, health/readiness handlers)

## Quickstart

Run the demo app:

```bash
make run
```

Then open:
- `GET http://localhost:8080/`
- `GET http://localhost:8080/health`
- `GET http://localhost:8080/api/health`
- `GET http://localhost:8080/ready`
- `GET http://localhost:8080/metrics`

Run tests:

```bash
make test
```

CLI examples:

```bash
go run ./cmd/penda help
go run ./cmd/penda routes
go run ./cmd/penda doctor
go run ./cmd/penda new myapp
```

## Official Examples

- `examples/hello`
- `examples/rest-api` (ORM CRUD + tests)
- `examples/web-app` (templates + static)

See: `docs/examples.md`

## Documentation

- Documentation index (recommended start): `docs/index.md`
- First part (foundation + core HTTP + routing + context): `docs/premiere-partie.md`
- Advanced phases (error handling, templates, config, blueprints, CLI, observability): `docs/advanced-phases.md`
- ORM integration (multi-SGBD + custom dialectors): `docs/orm.md`
- Official examples guide: `docs/examples.md`
- Full API reference (all packages): `docs/api-reference.md`
- Full implementation plan: `ROADMAP_MICRO_FRAMEWORK_GO.md`
- Release process: `RELEASE.md`
- Migration guide: `MIGRATION.md`
- Changelog: `CHANGELOG.md`

## Installation

The framework installation flow is documented here:
- `docs/premiere-partie.md` (section `2. Installation`)
