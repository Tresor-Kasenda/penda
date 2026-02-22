package orm

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
	fwerrors "penda/framework/errors"
)

type ormUser struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func TestOpenSQLiteAndAutoMigrate(t *testing.T) {
	db, err := Open(Config{
		Dialector:    "sqlite",
		DSN:          "file:orm-test?mode=memory&cache=shared",
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := AutoMigrate(db, &ormUser{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	if err := db.Create(&ormUser{Name: "Ada"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	var user ormUser
	if err := db.First(&user, "name = ?", "Ada").Error; err != nil {
		t.Fatalf("find user: %v", err)
	}
	if user.Name != "Ada" {
		t.Fatalf("expected name %q, got %q", "Ada", user.Name)
	}
}

func TestRegisterCustomDialector(t *testing.T) {
	name := "sqlite_alias_test"
	if err := RegisterDialector(name, func(dsn string) (gorm.Dialector, error) {
		return sqlite.Open(dsn), nil
	}); err != nil {
		t.Fatalf("register custom dialector: %v", err)
	}

	db, err := Open(Config{
		Dialector:    name,
		DSN:          "file:orm-test2?mode=memory&cache=shared",
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db, &ormUser{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
}

func TestMiddlewareInjectsDBIntoContext(t *testing.T) {
	db, err := Open(Config{
		Dialector:       "sqlite",
		DSN:             "file:orm-test3?mode=memory&cache=shared",
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db, &ormUser{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	server := fwapp.New()
	server.Use(Middleware(db))
	server.Get("/users", func(c *fwctx.Context) error {
		tx, ok := FromContext(c)
		if !ok {
			return fwerrors.Internal("db missing from context", nil)
		}
		if err := tx.Create(&ormUser{Name: "Grace"}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&ormUser{}).Count(&count).Error; err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]int64{"count": count})
	})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%q", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestUnsupportedDialector(t *testing.T) {
	_, err := Open(Config{
		Dialector: "not_real",
		DSN:       "ignored",
	})
	if err == nil {
		t.Fatal("expected error for unsupported dialector")
	}
}

func TestVersionedMigrationsAndRollback(t *testing.T) {
	db, err := Open(Config{
		Dialector:    "sqlite",
		DSN:          "file:orm-migrate-test?mode=memory&cache=shared",
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	migrations := []Migration{
		{
			Version: "001_create_widgets",
			Name:    "create widgets table",
			Up: func(tx *gorm.DB) error {
				return tx.Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`).Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec(`DROP TABLE widgets`).Error
			},
		},
		{
			Version: "002_seed_widget",
			Name:    "seed widget row",
			Up: func(tx *gorm.DB) error {
				return tx.Exec(`INSERT INTO widgets (id, name) VALUES (1, 'alpha')`).Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec(`DELETE FROM widgets WHERE id = 1`).Error
			},
		},
	}

	if err := Migrate(db, migrations...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Idempotency.
	if err := Migrate(db, migrations...); err != nil {
		t.Fatalf("migrate second pass: %v", err)
	}

	applied, err := AppliedMigrations(db)
	if err != nil {
		t.Fatalf("applied migrations: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(applied))
	}

	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM widgets`).Scan(&count).Error; err != nil {
		t.Fatalf("count widgets: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 widget row, got %d", count)
	}

	if err := RollbackLast(db, migrations...); err != nil {
		t.Fatalf("rollback last: %v", err)
	}

	applied, err = AppliedMigrations(db)
	if err != nil {
		t.Fatalf("applied migrations after rollback: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied migration after rollback, got %d", len(applied))
	}

	if err := db.Raw(`SELECT COUNT(*) FROM widgets`).Scan(&count).Error; err != nil {
		t.Fatalf("count widgets after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 widget rows after rollback, got %d", count)
	}
}

func TestPingAndPingCheck(t *testing.T) {
	db, err := Open(Config{
		Dialector:    "sqlite",
		DSN:          "file:orm-ping-test?mode=memory&cache=shared",
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := Ping(db); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := PingCheck(db)(); err != nil {
		t.Fatalf("ping check: %v", err)
	}
}
