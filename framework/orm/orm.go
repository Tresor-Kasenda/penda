package orm

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"

	fwapp "penda/framework/app"
	fwconfig "penda/framework/config"
	fwctx "penda/framework/context"
)

// Context key used to store a request-scoped ORM db session.
const ContextKey = "orm.db"

// DialectorOpener creates a gorm dialector from a DSN.
type DialectorOpener func(dsn string) (gorm.Dialector, error)

// Config contains ORM connection settings.
type Config struct {
	Dialector string
	DSN       string

	GormConfig *gorm.Config

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

var (
	registryMu sync.RWMutex
	registry   = map[string]DialectorOpener{}
)

func init() {
	MustRegisterDialector("sqlite", func(dsn string) (gorm.Dialector, error) {
		if strings.TrimSpace(dsn) == "" {
			dsn = "file::memory:?cache=shared"
		}
		return sqlite.Open(dsn), nil
	})
	MustRegisterDialector("postgres", func(dsn string) (gorm.Dialector, error) {
		return postgres.Open(dsn), nil
	})
	MustRegisterDialector("mysql", func(dsn string) (gorm.Dialector, error) {
		return mysql.Open(dsn), nil
	})
	MustRegisterDialector("sqlserver", func(dsn string) (gorm.Dialector, error) {
		return sqlserver.Open(dsn), nil
	})
}

// RegisterDialector registers a dialector opener under a unique name.
func RegisterDialector(name string, opener DialectorOpener) error {
	name = normalizeName(name)
	if name == "" {
		return errors.New("dialector name cannot be empty")
	}
	if opener == nil {
		return errors.New("dialector opener cannot be nil")
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[name]; exists {
		return fmt.Errorf("dialector %q is already registered", name)
	}
	registry[name] = opener
	return nil
}

// MustRegisterDialector registers a dialector or panics.
func MustRegisterDialector(name string, opener DialectorOpener) {
	if err := RegisterDialector(name, opener); err != nil {
		panic(err)
	}
}

// SupportedDialectors returns all registered dialector names.
func SupportedDialectors() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DefaultConfig returns a minimal ORM config.
func DefaultConfig() Config {
	return Config{
		Dialector: "sqlite",
		DSN:       "file::memory:?cache=shared",
	}
}

// FromFrameworkConfig maps framework config to ORM config.
func FromFrameworkConfig(cfg fwconfig.Config) Config {
	return Config{
		Dialector: cfg.DatabaseDriver,
		DSN:       cfg.DatabaseDSN,
	}
}

// OpenFromFrameworkConfig opens ORM DB using framework config database fields.
func OpenFromFrameworkConfig(cfg fwconfig.Config) (*gorm.DB, error) {
	return Open(FromFrameworkConfig(cfg))
}

// Open creates a gorm DB based on config and applies SQL pool settings.
func Open(cfg Config) (*gorm.DB, error) {
	cfg.Dialector = normalizeName(cfg.Dialector)
	if cfg.Dialector == "" {
		return nil, errors.New("dialector is required")
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, errors.New("dsn is required")
	}

	dialector, err := openDialector(cfg.Dialector, cfg.DSN)
	if err != nil {
		return nil, err
	}

	gormCfg := cfg.GormConfig
	if gormCfg == nil {
		gormCfg = &gorm.Config{}
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	return db, nil
}

// AutoMigrate runs GORM automigration.
func AutoMigrate(db *gorm.DB, models ...any) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}
	if len(models) == 0 {
		return errors.New("at least one model is required")
	}
	return db.AutoMigrate(models...)
}

// WithTransaction executes fn in a transaction.
func WithTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}
	if fn == nil {
		return errors.New("transaction function cannot be nil")
	}
	return db.Transaction(fn)
}

// Middleware injects request-scoped db session into framework context.
func Middleware(db *gorm.DB) fwapp.Middleware {
	if db == nil {
		panic("db cannot be nil")
	}

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			c.Set(ContextKey, db.WithContext(c.Request.Context()))
			return next(c)
		}
	}
}

// FromContext retrieves db session from framework context.
func FromContext(c *fwctx.Context) (*gorm.DB, bool) {
	if c == nil {
		return nil, false
	}

	value, ok := c.Get(ContextKey)
	if !ok {
		return nil, false
	}

	db, ok := value.(*gorm.DB)
	return db, ok
}

// MustFromContext retrieves db or panics if missing.
func MustFromContext(c *fwctx.Context) *gorm.DB {
	db, ok := FromContext(c)
	if !ok {
		panic("orm db is not set in request context")
	}
	return db
}

func openDialector(name, dsn string) (gorm.Dialector, error) {
	registryMu.RLock()
	opener, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported dialector %q (supported: %s)", name, strings.Join(SupportedDialectors(), ", "))
	}
	return opener(dsn)
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
