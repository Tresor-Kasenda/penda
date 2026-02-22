# Penda API Reference (Current Implementation)

This document summarizes the exported API for the current `penda` implementation.

It complements:
- `docs/premiere-partie.md` (intro + installation)
- `docs/advanced-phases.md` (phase-oriented overview)
- `docs/orm.md` (ORM-specific guide)
- `docs/examples.md` (official example apps)

## 1. Core Concepts

### Handler

```go
type Handler func(*fwctx.Context) error
```

Handlers return `error` to let the framework:
- map typed HTTP errors to status codes
- invoke status-specific and global error handlers
- choose default JSON or text/HTML error responses

### Middleware

```go
type Middleware func(next Handler) Handler
```

Middleware can:
- run before/after the next handler
- short-circuit by not calling `next`
- return an error (typed or generic)

### Error Handling Flow (High Level)

1. Route/middleware returns an error
2. Framework resolves status code (`err.StatusCode()` if available)
3. `OnStatus(code, handler)` is checked first
4. `OnError(...)` is called next
5. Fallback response is generated

Default fallback behavior:
- JSON (`{"error":"..."}`) when `Accept` includes JSON (or JSON request body with empty `Accept`)
- `http.NotFound` for 404
- `http.Error` otherwise

## 2. Package `framework/app`

Import:

```go
import "penda/framework/app"
```

### Types

- `type Handler func(*fwctx.Context) error`
- `type Middleware func(next Handler) Handler`
- `type ErrorHandler func(*fwctx.Context, error) error`
- `type Blueprint interface { Name() string; Mount(*App) }`
- `type RouteInfo struct { Method string; Path string }`

### App Lifecycle

- `app.New() *App`
- `(*App).Run(addr string) error`
  - uses configured address if `addr == ""`
- `(*App).Server(addr string) *http.Server`
- `(*App).ServeHTTP(w http.ResponseWriter, r *http.Request)`

### Config Integration

- `(*App).Config() config.Config`
- `(*App).SetConfig(cfg config.Config) error`
- `(*App).LoadConfig(opts config.ResolveOptions) error`
- `(*App).LoadConfigFromFile(path string) error`
- `(*App).LoadConfigFromEnv(prefix string) error`
- `(*App).SetMaxBodyBytes(n int64)`
- `(*App).MaxBodyBytes() int64`

Notes:
- `SetConfig` validates the config before applying it
- `LoadConfig` applies `config.Resolve` precedence (`defaults < profile < file < env`)
- `SetMaxBodyBytes` also updates the internal app config copy
- body limit is applied via `http.MaxBytesReader`

### Middleware Registration

- `(*App).Use(middlewares ...Middleware)`
- `(*Group).Use(middlewares ...Middleware)`

Execution order:
- global middleware wraps routing/dispatch
- group middleware wraps route handlers
- route-level middleware wraps the final handler

Practical consequence:
- global middleware can intercept `OPTIONS` preflight before route matching (e.g. CORS)
- global middleware also runs for 404/405 paths

### Error Handlers

- `(*App).OnError(handler ErrorHandler)`
- `(*App).OnStatus(code int, handler Handler)`

Behavior:
- `OnStatus` is evaluated before `OnError`
- if a status handler writes a response and returns `nil`, error handling stops
- if a handler returns a new error, processing continues with the new error

### Routing

Generic registration:
- `(*App).Handle(method, path string, handler Handler)`
- `(*App).HandleWith(method, path string, handler Handler, middlewares ...Middleware)`
- `(*Group).Handle(...)`
- `(*Group).HandleWith(...)`

Method shortcuts (App + Group):
- `Get`, `GetWith`
- `Post`, `PostWith`
- `Put`, `PutWith`
- `Patch`, `PatchWith`
- `Delete`, `DeleteWith`

Routing features:
- HTTP method-aware dispatch
- path params: `/users/:id`
- wildcard last segment only: `/files/*path`
- `404 Not Found`
- `405 Method Not Allowed` + `Allow` header

Route registration panics on invalid input:
- empty method
- invalid path (missing `/`, empty param name, non-final wildcard)
- nil handler
- nil middleware

### Groups

- `(*App).Group(prefix string, middlewares ...Middleware) *Group`
- `(*Group).Group(prefix string, middlewares ...Middleware) *Group`

Behavior:
- prefixes are normalized (leading slash added if needed)
- nested groups inherit parent middleware
- group prefixes compose safely

### Templates

- `(*App).SetTemplateFuncs(funcs template.FuncMap)`
- `(*App).SetTemplates(templates *template.Template)`
- `(*App).LoadTemplates(patterns ...string) error`

Behavior:
- `LoadTemplates` parses files matching one or more glob patterns
- template funcs registered via `SetTemplateFuncs` are applied when parsing
- `Context.Render(...)` requires templates to be loaded or set

### Static Files

- `(*App).Static(prefix, dir string)`
- `(*App).StaticWith(prefix, dir string, middlewares ...Middleware)`
- `(*Group).Static(prefix, dir string)`
- `(*Group).StaticWith(prefix, dir string, middlewares ...Middleware)`

Behavior:
- serves `GET` and `HEAD`
- sets `Cache-Control: public, max-age=3600`
- uses `http.FileServer` under the hood

### Blueprints and Diagnostics

- `(*App).Register(bp Blueprint)`
- `(*App).Routes() []RouteInfo`
- `(*App).MarshalJSON() ([]byte, error)`

`Routes()` is useful for:
- CLI route listing
- tests
- diagnostics and tooling

## 3. Package `framework/context`

Import:

```go
fwctx "penda/framework/context"
```

### Types

- `type Context struct`
- `type HTTPError struct`
- `type TemplateRenderer func(...) error`

### HTTP Errors

Constructors:
- `context.NewHTTPError(code, message string, err error) *HTTPError`
- `context.BadRequest(message string, err error) *HTTPError`

Methods:
- `(*HTTPError).Error() string`
- `(*HTTPError).Unwrap() error`
- `(*HTTPError).StatusCode() int`

These power typed status propagation across middleware/handlers.

### Context Creation

- `context.New(w http.ResponseWriter, r *http.Request, params map[string]string) *Context`

Usually created by the framework, not directly by app code.

### Request Accessors

- `(*Context).Param(key string) string`
- `(*Context).Query(key string) string`
- `(*Context).Header(key string) string`
- `(*Context).FormValue(key string) string`
- `(*Context).Cookie(name string) (*http.Cookie, error)`

### Locals (Request-Scoped Key/Value Storage)

- `(*Context).Set(key string, value any)`
- `(*Context).Get(key string) (any, bool)`

Common uses:
- request ID
- authenticated user
- request-scoped DB session (`framework/orm`)

### Response Helpers

- `(*Context).Status(code int)`
- `(*Context).StatusCode() int`
- `(*Context).SetHeader(key, value string)`
- `(*Context).SetCookie(cookie *http.Cookie)`
- `(*Context).Text(code int, body string) error`
- `(*Context).HTML(code int, body string) error`
- `(*Context).JSON(code int, payload any) error`
- `(*Context).Redirect(code int, location string)`
- `(*Context).File(path string) error`
- `(*Context).Download(path, filename string) error`

Template rendering:
- `(*Context).Render(name string, data any) error`
- `(*Context).RenderStatus(code int, name string, data any) error`

Notes:
- `File` and `Download` return typed HTTP errors when the file is missing/unreadable
- `Download` requires a non-empty filename

### Parsing / Binding

- `(*Context).BindJSON(dst any) error`
- `(*Context).BindForm(dst any) error`
- `(*Context).MultipartForm(maxMemory int64) (*multipart.Form, error)`
- `(*Context).FormFile(field string) (multipart.File, *multipart.FileHeader, error)`

`BindJSON` behavior:
- requires non-nil pointer
- rejects invalid JSON
- rejects multiple JSON values in the body
- applies basic required-field validation via `validate:"required"`

`BindForm` behavior:
- binds into a struct pointer
- supports `form` tags, fallback to `json` tags, then lowercased field names
- supports basic scalar types and slices
- applies `validate:"required"`

Validation caveat:
- current validation is intentionally simple (`required` only)

## 4. Package `framework/errors`

Import:

```go
fwerrors "penda/framework/errors"
```

This package is a convenience wrapper around `framework/context.HTTPError`.

### API

- `errors.New(code int, message string, err error)`
- `errors.BadRequest(...)`
- `errors.Unauthorized(...)`
- `errors.Forbidden(...)`
- `errors.NotFound(...)`
- `errors.Conflict(...)`
- `errors.TooManyRequests(...)`
- `errors.Internal(...)`

Use this package in handlers/services to keep status-coded errors readable.

## 5. Package `framework/middleware`

Import:

```go
import "penda/framework/middleware"
```

### Built-in Middleware

- `middleware.Recovery()`
  - converts panics to `500 internal server error`

- `middleware.Logger(logger *log.Logger)`
  - logs method, path, status, duration
  - uses `log.Default()` when `logger == nil`

- `middleware.RequestID()`
  - propagates or generates `X-Request-ID`
  - stores it in context locals under `request_id`

- `middleware.Timeout(timeout time.Duration)`
  - injects request context deadline
  - returns `504 Gateway Timeout` if handler times out without returning an error

- `middleware.CORS(config middleware.CORSConfig)`
  - sets CORS headers
  - short-circuits `OPTIONS` with `204 No Content`

- `middleware.SecurityHeaders(config middleware.SecurityHeadersConfig)`
  - sets sensible defaults for common security headers

- `middleware.RateLimit(config middleware.RateLimitConfig)`
  - in-memory fixed-window limiter
  - returns `429 Too Many Requests`
  - sets `Retry-After`

- `middleware.CSRF(config middleware.CSRFConfig)`
  - checks cookie token vs header/form token for unsafe methods
  - returns `403 Forbidden` on missing/invalid token

### Config Structs

- `CORSConfig`
  - `AllowOrigin`
  - `AllowMethods`
  - `AllowHeaders`
  - `AllowCredentials`
  - `MaxAgeSeconds`

- `SecurityHeadersConfig`
  - `ContentSecurityPolicy`
  - `ReferrerPolicy`
  - `XFrameOptions`
  - `XContentTypeOptions`
  - `PermissionsPolicy`
  - `StrictTransportSecurity`

- `RateLimitConfig`
  - `Requests`
  - `Window time.Duration`
  - `KeyFunc func(*fwctx.Context) string`

- `CSRFConfig`
  - `CookieName`
  - `HeaderName`
  - `FormField`

Important limitations:
- `RateLimit` is process-local/in-memory (not distributed)
- `CSRF` validates tokens but does not generate/store them

## 6. Package `framework/config`

Import:

```go
import "penda/framework/config"
```

### Type

- `type Config struct`
  - `Profile`
  - `Address`
  - `MaxBodyBytes`
  - `LogLevel`
  - `DatabaseDriver`
  - `DatabaseDSN`
- `type ResolveOptions struct`
  - `Profile`
  - `FilePath`
  - `EnvPrefix`

### API

- `config.Default() Config`
- `config.KnownProfiles() []string`
- `config.ProfileDefaults(profile string) (Config, error)`
- `config.LoadFile(path string) (Config, error)`
  - supports `.json`, `.yaml`, `.yml`, `.toml`
- `config.LoadEnv(prefix string) (Config, error)`
- `config.Resolve(opts ResolveOptions) (Config, error)`
  - strict precedence: `defaults < profile defaults < file < env`
- `config.Merge(base Config, overrides ...Config) Config`
- `(Config).Validate() error`

Environment variables (prefix optional):
- `PROFILE`
- `ADDRESS`
- `MAX_BODY_BYTES`
- `LOG_LEVEL`
- `DATABASE_DRIVER`
- `DATABASE_DSN`

With prefix `PENDA`:
- `PENDA_PROFILE`, `PENDA_ADDRESS`, ...

Validation notes:
- `Address` must be non-empty
- `MaxBodyBytes` must be `>= 0`
- `DatabaseDriver` and `DatabaseDSN` must be set together
- unknown profiles return an error

## 7. Package `framework/blueprint`

Import:

```go
import "penda/framework/blueprint"
```

### Type

- `type Blueprint struct`

### API

- `blueprint.New(name, prefix string, middlewares ...app.Middleware) *Blueprint`
- `(*Blueprint).Name() string`
- `(*Blueprint).Use(middlewares ...app.Middleware)`
- `(*Blueprint).SetTemplateFuncs(funcs template.FuncMap)`
- `(*Blueprint).LoadTemplates(patterns ...string)`
- `(*Blueprint).Static(prefix, dir string, middlewares ...app.Middleware)`
- `(*Blueprint).StaticWith(prefix, dir string, middlewares ...app.Middleware)`
- `(*Blueprint).Handle(method, path string, handler app.Handler, middlewares ...app.Middleware)`
- `(*Blueprint).Get(...)`
- `(*Blueprint).Post(...)`
- `(*Blueprint).Put(...)`
- `(*Blueprint).Patch(...)`
- `(*Blueprint).Delete(...)`
- `(*Blueprint).Mount(a *app.App)`

Helper:
- `blueprint.HealthBlueprint(prefix string) *Blueprint`

Blueprints implement `app.Blueprint`, so they can be mounted with:

```go
server.Register(bp)
```

Behavior notes:
- blueprint templates are merged into the app template set at mount time
- blueprint static mounts are scoped under the blueprint URL prefix

## 8. Package `framework/observability`

Import:

```go
import "penda/framework/observability"
```

### Metrics

- `observability.NewMetrics() *Metrics`
- `(*Metrics).Middleware() app.Middleware`
- `(*Metrics).Handler() app.Handler`

Metrics endpoint output is Prometheus text format (basic counters/gauge).

Exposed metric names include:
- `penda_requests_total`
- `penda_requests_in_flight`
- `penda_request_duration_ms_total`
- `penda_requests_by_method_status_total{...}`

### Health / Readiness Helpers

- `observability.HealthHandler() app.Handler`
- `observability.ReadinessHandler(check func() error) app.Handler`

Behavior:
- readiness returns `503 Service Unavailable` when `check()` fails

## 9. Package `framework/testing`

Import:

```go
fwtest "penda/framework/testing"
```

### Client

- `testing.NewClient(handler http.Handler) *Client`
- `(*Client).Do(req *http.Request) *Response`
- `(*Client).DoWithCookies(req *http.Request, cookies ...*http.Cookie) *Response`
- `(*Client).Get(path string) *Response`
- `(*Client).PostJSON(path string, payload any) *Response`
- `(*Client).PostForm(path string, values url.Values) *Response`
- `(*Client).PostMultipart(path string, fields map[string]string, files ...testing.MultipartFile) *Response`

### Multipart Helper Types

- `type MultipartFile struct`
  - `FieldName`
  - `FileName`
  - `Content`
  - `ContentType`

### Response Helpers

- `(*Response).StatusCode() int`
- `(*Response).Header(key string) string`
- `(*Response).BodyString() string`
- `(*Response).BodyBytes() []byte`
- `(*Response).Result() *http.Response`
- `(*Response).Cookies() []*http.Cookie`
- `(*Response).Cookie(name string) (*http.Cookie, bool)`
- `(*Response).DecodeJSON(dst any) error`

### Assertions

- `testing.AssertStatus(t, response, expected)`
- `testing.AssertHeaderContains(t, response, key, contains)`
- `testing.AssertBodyContains(t, response, contains)`
- `testing.AssertJSONEqual(t, response, expected)`
- `testing.AssertCookieValue(t, response, name, expectedValue)`
- `testing.ReadAll(t, r io.Reader) []byte`

## 10. Package `framework/orm`

Import:

```go
import "penda/framework/orm"
```

This package wraps GORM and exposes a framework-friendly integration layer.

### Built-in Dialectors

Registered by default:
- `sqlite`
- `postgres`
- `mysql`
- `sqlserver`

### Types

- `type DialectorOpener func(dsn string) (gorm.Dialector, error)`
- `type Config struct`
  - `Dialector`
  - `DSN`
  - `GormConfig *gorm.Config`
  - pool settings (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime`)

### Dialector Registry

- `orm.RegisterDialector(name string, opener DialectorOpener) error`
- `orm.MustRegisterDialector(name string, opener DialectorOpener)`
- `orm.SupportedDialectors() []string`

Use this to support other SQL databases (custom GORM dialector).

### DB Opening / Migrations / Transactions

- `orm.DefaultConfig() Config`
- `orm.FromFrameworkConfig(cfg config.Config) Config`
- `orm.OpenFromFrameworkConfig(cfg config.Config) (*gorm.DB, error)`
- `orm.Open(cfg orm.Config) (*gorm.DB, error)`
- `orm.AutoMigrate(db *gorm.DB, models ...any) error`
- `orm.WithTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error`

### Request-Scoped DB Injection

- `const orm.ContextKey = "orm.db"`
- `orm.Middleware(db *gorm.DB) app.Middleware`
- `orm.FromContext(c *fwctx.Context) (*gorm.DB, bool)`
- `orm.MustFromContext(c *fwctx.Context) *gorm.DB`

See `docs/orm.md` for examples and configuration patterns.

## 11. CLI (`cmd/penda` + `internal/cli`)

CLI command entrypoint:
- `cmd/penda/main.go`

Command implementation:
- `internal/cli/cli.go`

### Commands

- `penda new <name>`
  - creates a minimal project scaffold (`main.go`, `go.mod`, `README.md`)

- `penda run [addr]`
  - runs the built-in demo app

- `penda routes`
  - prints routes of the built-in demo app

- `penda doctor`
  - prints diagnostics (`Go`, `OS/Arch`, working directory)

### Demo App Routes (Current)

Built-in demo app includes:
- `GET /`
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `GET /api/health` (blueprint)
- `POST /api/echo`

## 12. End-to-End Example (Feature Composition)

```go
package main

import (
    "log"
    "net/http"
    "time"

    "penda/framework/app"
    fwctx "penda/framework/context"
    "penda/framework/middleware"
    "penda/framework/observability"
)

func main() {
    server := app.New()
    metrics := observability.NewMetrics()

    server.Use(
        middleware.Recovery(),
        middleware.RequestID(),
        middleware.Logger(log.Default()),
        middleware.Timeout(5*time.Second),
        middleware.CORS(middleware.CORSConfig{}),
        middleware.SecurityHeaders(middleware.SecurityHeadersConfig{}),
        metrics.Middleware(),
    )

    server.Get("/health", observability.HealthHandler())
    server.Get("/metrics", metrics.Handler())

    api := server.Group("/api")
    api.Get("/hello/:name", func(c *fwctx.Context) error {
        return c.JSON(http.StatusOK, map[string]string{
            "hello": c.Param("name"),
        })
    })

    log.Fatal(server.Run(":8080"))
}
```

## 13. Current Limitations (Documented)

- Template auto-reload is not implemented yet
- Config file loading currently supports JSON only
- Rate limiting is in-memory (single process)
- CSRF middleware validates tokens but does not generate/manage them
- ORM integration targets SQL databases through GORM dialectors
- No OpenTelemetry tracing integration yet
