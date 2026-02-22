# Official Examples

`penda` includes three official examples under `examples/`.

## 1. Hello API (`examples/hello`)

What it demonstrates:
- basic app setup
- middleware registration
- path params
- health endpoint

Run:

```bash
go run ./examples/hello
```

Endpoints:
- `GET http://localhost:8081/`
- `GET http://localhost:8081/health`
- `GET http://localhost:8081/hello/scott`

## 2. REST API + ORM CRUD (`examples/rest-api`)

What it demonstrates:
- GORM integration (`framework/orm`)
- migrations (`AutoMigrate`)
- request-scoped DB middleware
- full CRUD routes
- health/readiness/metrics endpoints

Run:

```bash
go run ./examples/rest-api
```

Endpoints:
- `GET http://localhost:8083/health`
- `GET http://localhost:8083/ready`
- `GET http://localhost:8083/metrics`
- `GET http://localhost:8083/api/users`
- `POST http://localhost:8083/api/users`
- `GET http://localhost:8083/api/users/:id`
- `PATCH http://localhost:8083/api/users/:id`
- `DELETE http://localhost:8083/api/users/:id`

Run tests:

```bash
go test ./examples/rest-api
```

## 3. Web App (`examples/web-app`)

What it demonstrates:
- server-rendered template (`Context.Render`)
- static assets (`app.Static`)
- security headers middleware

Run from the example directory (recommended because templates/static paths are relative):

```bash
cd examples/web-app
go run .
```

Endpoints:
- `GET http://localhost:8082/`
- `GET http://localhost:8082/ping`
