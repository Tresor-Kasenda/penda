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
const migrationsTableName = "penda_schema_migrations"

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

// Migration represents a versioned schema/data migration.
type Migration struct {
	Version string
	Name    string
	Up      func(tx *gorm.DB) error
	Down    func(tx *gorm.DB) error
}

// AppliedMigration describes a migration row persisted in the schema migrations table.
type AppliedMigration struct {
	Version   string
	Name      string
	AppliedAt time.Time
}

type migrationRecord struct {
	Version   string    `gorm:"primaryKey;size:191"`
	Name      string    `gorm:"size:255;not null"`
	AppliedAt time.Time `gorm:"not null"`
}

func (migrationRecord) TableName() string {
	return migrationsTableName
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

// Migrate applies versioned migrations in ascending version order.
func Migrate(db *gorm.DB, migrations ...Migration) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}
	ordered, err := normalizeMigrations(migrations)
	if err != nil {
		return err
	}
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	applied, err := appliedMigrationMap(db)
	if err != nil {
		return err
	}

	for _, migration := range ordered {
		if _, ok := applied[migration.Version]; ok {
			continue
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return err
			}
			return tx.Table(migrationsTableName).Create(&migrationRecord{
				Version:   migration.Version,
				Name:      migration.Name,
				AppliedAt: time.Now().UTC(),
			}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %s (%s): %w", migration.Version, migration.Name, err)
		}
	}

	return nil
}

// AppliedMigrations returns migrations stored in the schema migrations table.
func AppliedMigrations(db *gorm.DB) ([]AppliedMigration, error) {
	if db == nil {
		return nil, errors.New("db cannot be nil")
	}
	if err := ensureMigrationsTable(db); err != nil {
		return nil, err
	}

	var rows []migrationRecord
	if err := db.Table(migrationsTableName).Order("version asc").Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]AppliedMigration, 0, len(rows))
	for _, row := range rows {
		out = append(out, AppliedMigration{
			Version:   row.Version,
			Name:      row.Name,
			AppliedAt: row.AppliedAt,
		})
	}
	return out, nil
}

// RollbackLast rolls back the most recently applied migration (by version order)
// using the provided migration registry.
func RollbackLast(db *gorm.DB, migrations ...Migration) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}
	ordered, err := normalizeMigrations(migrations)
	if err != nil {
		return err
	}
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	applied, err := AppliedMigrations(db)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		return nil
	}
	last := applied[len(applied)-1]

	lookup := map[string]Migration{}
	for _, migration := range ordered {
		lookup[migration.Version] = migration
	}
	migration, ok := lookup[last.Version]
	if !ok {
		return fmt.Errorf("migration %q not found in registry", last.Version)
	}
	if migration.Down == nil {
		return fmt.Errorf("migration %q has no down function", last.Version)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := migration.Down(tx); err != nil {
			return err
		}
		return tx.Table(migrationsTableName).Where("version = ?", last.Version).Delete(&migrationRecord{}).Error
	})
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

// Ping verifies that the database connection is alive.
func Ping(db *gorm.DB) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// PingCheck returns a readiness/health check function compatible with observability.ReadinessHandler.
func PingCheck(db *gorm.DB) func() error {
	return func() error {
		return Ping(db)
	}
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

func ensureMigrationsTable(db *gorm.DB) error {
	return db.AutoMigrate(&migrationRecord{})
}

func appliedMigrationMap(db *gorm.DB) (map[string]AppliedMigration, error) {
	rows, err := AppliedMigrations(db)
	if err != nil {
		return nil, err
	}
	out := make(map[string]AppliedMigration, len(rows))
	for _, row := range rows {
		out[row.Version] = row
	}
	return out, nil
}

func normalizeMigrations(migrations []Migration) ([]Migration, error) {
	ordered := make([]Migration, 0, len(migrations))
	seen := map[string]struct{}{}
	for _, migration := range migrations {
		version := strings.TrimSpace(migration.Version)
		name := strings.TrimSpace(migration.Name)
		if version == "" {
			return nil, errors.New("migration version cannot be empty")
		}
		if name == "" {
			return nil, fmt.Errorf("migration %q name cannot be empty", version)
		}
		if migration.Up == nil {
			return nil, fmt.Errorf("migration %q up function cannot be nil", version)
		}
		if _, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %q", version)
		}
		seen[version] = struct{}{}

		migration.Version = version
		migration.Name = name
		ordered = append(ordered, migration)
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Version < ordered[j].Version
	})
	return ordered, nil
}
