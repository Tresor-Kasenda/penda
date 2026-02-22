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

func TestLoadFileYAMLAndTOML(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		data := []byte("" +
			"profile: test\n" +
			"database_driver: sqlite\n" +
			"database_dsn: file::memory:?cache=shared\n")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if cfg.Profile != "test" {
			t.Fatalf("expected profile %q, got %q", "test", cfg.Profile)
		}
		if cfg.Address != ":0" {
			t.Fatalf("expected test profile address %q, got %q", ":0", cfg.Address)
		}
		if cfg.LogLevel != "warn" {
			t.Fatalf("expected test profile log level %q, got %q", "warn", cfg.LogLevel)
		}
	})

	t.Run("toml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		data := []byte("" +
			"profile = \"prod\"\n" +
			"address = \":9090\"\n" +
			"database_driver = \"sqlite\"\n" +
			"database_dsn = \"file:prod.db\"\n")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if cfg.Profile != "prod" || cfg.Address != ":9090" {
			t.Fatalf("unexpected config: %+v", cfg)
		}
		if cfg.LogLevel != "warn" {
			t.Fatalf("expected prod default log level %q, got %q", "warn", cfg.LogLevel)
		}
	})
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

func TestProfileDefaults(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		address  string
		logLevel string
	}{
		{name: "dev", profile: "dev", address: ":8080", logLevel: "debug"},
		{name: "test", profile: "test", address: ":0", logLevel: "warn"},
		{name: "prod", profile: "prod", address: ":8080", logLevel: "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ProfileDefaults(tt.profile)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			if cfg.Profile != tt.profile || cfg.Address != tt.address || cfg.LogLevel != tt.logLevel {
				t.Fatalf("unexpected profile defaults: %+v", cfg)
			}
		})
	}
}

func TestResolvePrecedenceAndZeroOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte("" +
		"profile: prod\n" +
		"address: \":9090\"\n" +
		"max_body_bytes: 0\n" +
		"database_driver: sqlite\n" +
		"database_dsn: file:prod.db\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("PENDA_ADDRESS", ":7070")
	t.Setenv("PENDA_LOG_LEVEL", "error")

	cfg, err := Resolve(ResolveOptions{
		FilePath:  path,
		EnvPrefix: "PENDA",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if cfg.Profile != "prod" {
		t.Fatalf("expected profile %q, got %q", "prod", cfg.Profile)
	}
	if cfg.Address != ":7070" {
		t.Fatalf("expected env address override %q, got %q", ":7070", cfg.Address)
	}
	if cfg.MaxBodyBytes != 0 {
		t.Fatalf("expected file zero override for max_body_bytes, got %d", cfg.MaxBodyBytes)
	}
	if cfg.LogLevel != "error" {
		t.Fatalf("expected env log level override %q, got %q", "error", cfg.LogLevel)
	}
	if cfg.DatabaseDriver != "sqlite" || cfg.DatabaseDSN != "file:prod.db" {
		t.Fatalf("unexpected database config: %+v", cfg)
	}
}

func TestResolveExplicitProfileHasPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"profile":"test"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("PENDA_PROFILE", "dev")

	cfg, err := Resolve(ResolveOptions{
		Profile:   "prod",
		FilePath:  path,
		EnvPrefix: "PENDA",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if cfg.Profile != "prod" {
		t.Fatalf("expected explicit profile %q to win, got %q", "prod", cfg.Profile)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("expected prod defaults log level %q, got %q", "warn", cfg.LogLevel)
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

func TestResolveRejectsUnknownProfile(t *testing.T) {
	if _, err := Resolve(ResolveOptions{Profile: "staging"}); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}
