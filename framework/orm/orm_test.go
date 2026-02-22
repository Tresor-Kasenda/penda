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
