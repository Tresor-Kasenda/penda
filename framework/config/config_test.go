package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{"profile":"prod","address":":9090","max_body_bytes":2048,"log_level":"warn","database_driver":"sqlite","database_dsn":"file:test.db"}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if cfg.Profile != "prod" || cfg.Address != ":9090" || cfg.MaxBodyBytes != 2048 || cfg.LogLevel != "warn" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.DatabaseDriver != "sqlite" || cfg.DatabaseDSN != "file:test.db" {
		t.Fatalf("unexpected database config: %+v", cfg)
	}
}

func TestLoadEnvWithPrefix(t *testing.T) {
	t.Setenv("PENDA_PROFILE", "test")
	t.Setenv("PENDA_ADDRESS", ":7070")
	t.Setenv("PENDA_MAX_BODY_BYTES", "4096")
	t.Setenv("PENDA_LOG_LEVEL", "debug")
	t.Setenv("PENDA_DATABASE_DRIVER", "postgres")
	t.Setenv("PENDA_DATABASE_DSN", "postgres://user:pass@localhost/db")

	cfg, err := LoadEnv("PENDA")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if cfg.Profile != "test" || cfg.Address != ":7070" || cfg.MaxBodyBytes != 4096 || cfg.LogLevel != "debug" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.DatabaseDriver != "postgres" || cfg.DatabaseDSN != "postgres://user:pass@localhost/db" {
		t.Fatalf("unexpected database config: %+v", cfg)
	}
}

func TestMerge(t *testing.T) {
	base := Default()
	override := Config{
		Address:        ":9999",
		MaxBodyBytes:   1234,
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file::memory:?cache=shared",
	}

	merged := Merge(base, override)
	if merged.Address != ":9999" || merged.MaxBodyBytes != 1234 {
		t.Fatalf("unexpected merged config: %+v", merged)
	}
	if merged.DatabaseDriver != "sqlite" || merged.DatabaseDSN != "file::memory:?cache=shared" {
		t.Fatalf("unexpected merged database config: %+v", merged)
	}
}

func TestValidateDatabaseFieldsMustBePaired(t *testing.T) {
	cfg := Default()
	cfg.DatabaseDriver = "sqlite"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when dsn is missing")
	}

	cfg = Default()
	cfg.DatabaseDSN = "file::memory:?cache=shared"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when driver is missing")
	}
}
