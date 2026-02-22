# Advanced Phases (6 to 12)

This document describes what is implemented after the initial core.

## Phase 6: Centralized Error Handling

Implemented:
- `framework/context.HTTPError` with `StatusCode()`
- `app.OnError(func(*Context, error) error)`
- `app.OnStatus(code, handler)`
- automatic HTTP status mapping from typed errors

Example:

```go
server.OnStatus(http.StatusNotFound, func(c *fwctx.Context) error {
    return c.JSON(http.StatusNotFound, map[string]string{"error": "resource not found"})
})

server.OnError(func(c *fwctx.Context, err error) error {
    return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
})
```

## Phase 7: Templates and Static Files

Implemented:
- `app.LoadTemplates(patterns...)`
- `Context.Render(name, data)` and `Context.RenderStatus(...)`
- `app.Static(prefix, dir)` and `group.Static(prefix, dir)`

Example:

```go
if err := server.LoadTemplates("templates/*.tmpl"); err != nil {
    panic(err)
}

server.Get("/home", func(c *fwctx.Context) error {
    return c.Render("home.tmpl", map[string]string{"Title": "Penda"})
})

server.Static("/assets", "./public")
```

## Phase 8: Config and Environment

Implemented in `framework/config`:
- `config.Default()`
- `config.LoadFile(path)` (JSON)
- `config.LoadEnv(prefix)`
- `config.Merge(base, overrides...)`

Integrated in `App`:
- `app.Config()`
- `app.SetConfig(cfg)`
- `app.LoadConfigFromFile(path)`
- `app.LoadConfigFromEnv(prefix)`

## Phase 9: Blueprints / Modules

Implemented in `framework/blueprint`:
- `blueprint.New(name, prefix, middleware...)`
- route registration methods (`Get`, `Post`, `Put`, `Patch`, `Delete`)
- `app.Register(bp)`

Example:

```go
bp := blueprint.New("users", "/api")
bp.Get("/users/:id", func(c *fwctx.Context) error {
    return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
})
server.Register(bp)
```

## Phase 10: CLI Developer Experience

Implemented commands:
- `penda new <name>`
- `penda run [addr]`
- `penda routes`
- `penda doctor`

Entrypoint:
- `cmd/penda/main.go`

Core CLI logic:
- `internal/cli/cli.go`

## Phase 11: Testing Toolkit

Implemented in `framework/testing`:
- `testing.NewClient(handler)`
- `client.Get(path)`
- `client.PostJSON(path, payload)`
- `client.PostForm(path, values)`
- helpers: `AssertStatus`, `AssertHeaderContains`

## Phase 12: Observability and Security

Observability (`framework/observability`):
- in-memory metrics recorder middleware
- Prometheus-style `/metrics` handler
- `HealthHandler()`
- `ReadinessHandler(check func() error)`

Security middleware (`framework/middleware`):
- `SecurityHeaders`
- `RateLimit`
- `CSRF`

## What is still partial

- template auto-reload in dev mode is not implemented yet
- no OpenTelemetry tracing integration yet
- rate limiting is in-memory only (single process)
- config file format currently supports JSON (YAML/TOML can be added later)

