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
- Config package (`framework/config`) with file/env loaders
- Blueprints/modules (`framework/blueprint`)
- CLI commands (`penda new`, `penda run`, `penda routes`, `penda doctor`)
- Testing toolkit (`framework/testing`)
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

## Documentation

- First part (foundation + core HTTP + routing + context): `docs/premiere-partie.md`
- Advanced phases (error handling, templates, config, blueprints, CLI, observability): `docs/advanced-phases.md`
- Full implementation plan: `ROADMAP_MICRO_FRAMEWORK_GO.md`

## Installation

The framework installation flow is documented here:
- `docs/premiere-partie.md` (section `2. Installation`)
