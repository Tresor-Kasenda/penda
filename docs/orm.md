# ORM Integration (Multi-Database)

`penda` now includes an ORM integration layer built on top of [GORM](https://gorm.io/) in:

- `framework/orm`

The goal is:
- support common SQL databases out of the box
- allow custom database engines through dialector registration
- keep the framework database-agnostic at the app layer

## Supported Databases (Built-in)

Built-in dialectors are registered automatically:
- `sqlite`
- `postgres`
- `mysql`
- `sqlserver`

You can list them:

```go
dialectors := orm.SupportedDialectors()
```

## Framework Config Integration

The framework config now supports database fields:
- `database_driver`
- `database_dsn`

Environment variables (with prefix support):
- `PENDA_DATABASE_DRIVER`
- `PENDA_DATABASE_DSN`

Example:

```bash
export PENDA_DATABASE_DRIVER=postgres
export PENDA_DATABASE_DSN='postgres://user:pass@localhost:5432/appdb?sslmode=disable'
```

## Open a Database Connection

### From explicit ORM config

```go
db, err := orm.Open(orm.Config{
    Dialector: "sqlite",
    DSN:       "file:app.db",
})
if err != nil {
    panic(err)
}
```

### From framework config

```go
cfg, err := config.LoadEnv("PENDA")
if err != nil {
    panic(err)
}

db, err := orm.OpenFromFrameworkConfig(cfg)
if err != nil {
    panic(err)
}
```

## Auto-Migrate Models

```go
type User struct {
    ID   uint `gorm:"primaryKey"`
    Name string
}

if err := orm.AutoMigrate(db, &User{}); err != nil {
    panic(err)
}
```

## Versioned Migrations (Recommended for Real Apps)

In addition to `AutoMigrate`, the ORM package now provides a simple versioned migration runner.

```go
migrations := []orm.Migration{
    {
        Version: "001_create_users",
        Name:    "create users table",
        Up: func(tx *gorm.DB) error {
            return tx.AutoMigrate(&User{})
        },
        Down: func(tx *gorm.DB) error {
            return tx.Migrator().DropTable(&User{})
        },
    },
}

if err := orm.Migrate(db, migrations...); err != nil {
    panic(err)
}
```

Available helpers:
- `orm.Migrate(...)`
- `orm.AppliedMigrations(db)`
- `orm.RollbackLast(db, migrations...)`

Migration metadata is stored in the `penda_schema_migrations` table.

## Inject DB into Request Context (Middleware)

Use the ORM middleware to inject a request-scoped GORM session:

```go
server.Use(orm.Middleware(db))
```

Then retrieve it in handlers:

```go
server.Get("/users", func(c *fwctx.Context) error {
    db, ok := orm.FromContext(c)
    if !ok {
        return fwerrors.Internal("database is not available", nil)
    }

    var users []User
    if err := db.Find(&users).Error; err != nil {
        return err
    }

    return c.JSON(http.StatusOK, users)
})
```

If you prefer strict behavior:

```go
db := orm.MustFromContext(c)
```

## Transactions

```go
err := orm.WithTransaction(db, func(tx *gorm.DB) error {
    if err := tx.Create(&User{Name: "Ada"}).Error; err != nil {
        return err
    }
    return nil
})
```

## DB Health / Readiness Checks

Use the ORM ping helpers with observability:

```go
server.Get("/ready", observability.ReadinessHandler(orm.PingCheck(db)))
```

Helpers:
- `orm.Ping(db)`
- `orm.PingCheck(db)`

## Support Any Database (Custom Dialector)

If your database is not one of the built-ins, register a custom dialector.

This is the extension point that makes the integration work with *any* SGBD, as long as you have a GORM dialector (or can build one).

```go
orm.MustRegisterDialector("mydb", func(dsn string) (gorm.Dialector, error) {
    // Replace with your own driver/dialector constructor.
    return mydialector.Open(dsn), nil
})

db, err := orm.Open(orm.Config{
    Dialector: "mydb",
    DSN:       "mydb://...",
})
```

Notes:
- registration names are case-insensitive (`"MyDB"` and `"mydb"` normalize to the same key)
- duplicate registrations return an error (or panic with `MustRegisterDialector`)

## Pool Tuning (Optional)

`orm.Config` supports SQL pool settings:
- `MaxOpenConns`
- `MaxIdleConns`
- `ConnMaxLifetime`
- `ConnMaxIdleTime`

Example:

```go
db, err := orm.Open(orm.Config{
    Dialector:        "postgres",
    DSN:              "...",
    MaxOpenConns:     25,
    MaxIdleConns:     10,
    ConnMaxLifetime:  time.Hour,
    ConnMaxIdleTime:  15 * time.Minute,
})
```

## Files

- `framework/orm/orm.go`
- `framework/orm/orm_test.go`
- `framework/config/config.go` (DB fields + env/file support)
