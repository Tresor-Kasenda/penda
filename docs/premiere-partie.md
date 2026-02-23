# Documentation - Framework First Part

This document covers what is already implemented:
- Phase 0: project foundations.
- Phase 1: minimal HTTP kernel.
- Start of Phase 2: method-aware routing with params and wildcard.

## 1. Current Scope

Available features:
- `App` type with `New()`, `Run(addr)`, `ServeHTTP(...)`.
- Route registration with `Handle(...)` and helpers `Get/Post/Put/Delete/Patch`.
- Route matching by HTTP method.
- Path params with `:param`.
- Path wildcard with `*param` (only allowed as last segment).
- `Context` type with:
  - request/response access
  - `Param`, `Query`, `Header`
  - local storage through `Set`/`Get`
  - response helpers `Text` and `JSON`
- `404 Not Found` handling.
- `405 Method Not Allowed` handling with `Allow` header.

## 2. Installation

### 2.1 Prerequisites

- Go 1.22 or higher

### 2.2 Create a New Project

```bash
mkdir myapi && cd myapi
go mod init myapi
```

### 2.3 Install Penda

Option A (when this framework is published on a Git host):

```bash
go get github.com/Tresor-Kasenda/penda
```

Option B (current local development setup):

```bash
PENDA_PATH="/absolute/path/to/penda"
go mod edit -replace penda=$PENDA_PATH
go get penda@latest
```

You are now ready to build an API with `penda`.

### 2.4 Verify Installation

Create a simple `main.go`:

```go
package main

import (
    "net/http"

    "penda/framework/app"
    fwctx "penda/framework/context"
)

func main() {
    server := app.New()

    server.Get("/", func(c *fwctx.Context) error {
        return c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from penda!",
        })
    })

    if err := server.Run(":8080"); err != nil {
        panic(err)
    }
}
```

Run the server:

```bash
go run main.go
```

Visit `http://localhost:8080` to verify everything works.

## 3. Current Public API

## 3.1 Package `framework/app`

Create an application:

Minimal imports:

```go
import (
    "penda/framework/app"
    fwctx "penda/framework/context"
)
```

```go
server := app.New()
```

Register routes:

```go
server.Handle("GET", "/health", func(c *fwctx.Context) error {
    return c.Text(200, "ok")
})

server.Get("/users/:id", handler)
server.Post("/users", handler)
server.Put("/users/:id", handler)
server.Delete("/users/:id", handler)
server.Patch("/users/:id", handler)
```

Handler signature:

```go
type Handler func(*fwctx.Context) error
```

Run server:

```go
if err := server.Run(":8080"); err != nil {
    log.Fatal(err)
}
```

## 3.2 Package `framework/context`

`Context` fields:
- `Writer http.ResponseWriter`
- `Request *http.Request`
- `Params map[string]string`
- `Locals map[string]any`

Methods:
- `Param(key string) string`
- `Query(key string) string`
- `Header(key string) string`
- `Set(key string, value any)`
- `Get(key string) (any, bool)`
- `Text(code int, body string) error`
- `JSON(code int, payload any) error`

## 4. Routing Rules

Static route:

```go
server.Get("/health", handler)
```

Route with param:

```go
server.Get("/users/:id", func(c *fwctx.Context) error {
    return c.Text(200, c.Param("id"))
})
```

Wildcard route:

```go
server.Get("/files/*path", func(c *fwctx.Context) error {
    return c.Text(200, c.Param("path"))
})
```

Important rules:
- The path must start with `/`.
- `:param` must have a name (`/:` is invalid).
- `*param` must have a name (`/*` is invalid).
- `*param` must be the last segment.
- Empty method or `nil` handler panics at registration time.

## 5. HTTP Behavior

When a route does not exist:
- response is `404 Not Found`.

When the path exists but the method does not:
- response is `405 Method Not Allowed`.
- `Allow` header contains supported methods for this path.

Handler error behavior:
- If a handler returns an error before writing a response:
  - response is `500 Internal Server Error` with the error message.
- If a handler already wrote the response:
  - written status is kept (no override to 500).

## 6. Full Example

Minimal example based on `cmd/penda/main.go`:

```go
package main

import (
    "log"
    "net/http"

    "penda/framework/app"
    fwctx "penda/framework/context"
)

func main() {
    server := app.New()

    server.Get("/health", func(c *fwctx.Context) error {
        return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
    })

    server.Get("/users/:id/files/*path", func(c *fwctx.Context) error {
        return c.JSON(http.StatusOK, map[string]string{
            "id": c.Param("id"),
            "path": c.Param("path"),
        })
    })

    log.Println("penda listening on :8080")
    if err := server.Run(":8080"); err != nil {
        log.Fatal(err)
    }
}
```

## 7. Tests and Quality

Current test coverage includes:
- successful GET route.
- `404`.
- `405` + `Allow`.
- `:id` and `*path` extraction.
- handler errors before/after response write.
- `Context` local storage (`Set`/`Get`) and `Query`.

Useful commands:

```bash
make test
make lint
make bench
```

## 8. Current Limitations

Not implemented yet:
- middleware (global/group/route).
- route groups.
- JSON/form/multipart body parsing.
- advanced response helpers (redirect, file, status builder).
- customizable centralized error handling.
- templates/static/config/CLI.

These are planned in upcoming roadmap phases.
