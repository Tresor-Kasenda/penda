package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// Environment variable names without prefix.
	envProfile        = "PROFILE"
	envAddress        = "ADDRESS"
	envMaxBodyBytes   = "MAX_BODY_BYTES"
	envLogLevel       = "LOG_LEVEL"
	envDatabaseDriver = "DATABASE_DRIVER"
	envDatabaseDSN    = "DATABASE_DSN"
)

// Config holds runtime configuration.
type Config struct {
	Profile        string `json:"profile"`
	Address        string `json:"address"`
	MaxBodyBytes   int64  `json:"max_body_bytes"`
	LogLevel       string `json:"log_level"`
	DatabaseDriver string `json:"database_driver"`
	DatabaseDSN    string `json:"database_dsn"`
}

// Default returns default runtime configuration.
func Default() Config {
	return Config{
		Profile:        "dev",
		Address:        ":8080",
		MaxBodyBytes:   1 << 20,
		LogLevel:       "info",
		DatabaseDriver: "",
		DatabaseDSN:    "",
	}
}

// LoadFile loads configuration from a JSON file.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}

// LoadEnv loads configuration from environment variables.
// Prefix is optional; when provided, variables are looked up as PREFIX_<KEY>.
func LoadEnv(prefix string) (Config, error) {
	cfg := Default()

	prefix = strings.TrimSpace(prefix)
	if prefix != "" {
		prefix = strings.ToUpper(strings.TrimSuffix(prefix, "_")) + "_"
	}

	if profile := os.Getenv(prefix + envProfile); profile != "" {
		cfg.Profile = profile
	}
	if address := os.Getenv(prefix + envAddress); address != "" {
		cfg.Address = address
	}
	if logLevel := os.Getenv(prefix + envLogLevel); logLevel != "" {
		cfg.LogLevel = logLevel
	}
	if dbDriver := os.Getenv(prefix + envDatabaseDriver); dbDriver != "" {
		cfg.DatabaseDriver = dbDriver
	}
	if dbDSN := os.Getenv(prefix + envDatabaseDSN); dbDSN != "" {
		cfg.DatabaseDSN = dbDSN
	}
	if raw := os.Getenv(prefix + envMaxBodyBytes); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s%s: %w", prefix, envMaxBodyBytes, err)
		}
		cfg.MaxBodyBytes = value
	}

	return cfg, nil
}

// Merge overlays non-zero/nonnull override values onto base.
func Merge(base Config, overrides ...Config) Config {
	result := base

	for _, override := range overrides {
		if strings.TrimSpace(override.Profile) != "" {
			result.Profile = override.Profile
		}
		if strings.TrimSpace(override.Address) != "" {
			result.Address = override.Address
		}
		if override.MaxBodyBytes != 0 {
			result.MaxBodyBytes = override.MaxBodyBytes
		}
		if strings.TrimSpace(override.LogLevel) != "" {
			result.LogLevel = override.LogLevel
		}
		if strings.TrimSpace(override.DatabaseDriver) != "" {
			result.DatabaseDriver = override.DatabaseDriver
		}
		if strings.TrimSpace(override.DatabaseDSN) != "" {
			result.DatabaseDSN = override.DatabaseDSN
		}
	}

	return result
}

// Validate checks critical configuration invariants.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("address cannot be empty")
	}
	if c.MaxBodyBytes < 0 {
		return errors.New("max_body_bytes must be >= 0")
	}
	driver := strings.TrimSpace(c.DatabaseDriver)
	dsn := strings.TrimSpace(c.DatabaseDSN)
	if (driver == "") != (dsn == "") {
		return errors.New("database_driver and database_dsn must be set together")
	}
	return nil
}
