package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
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
	Profile        string `json:"profile" yaml:"profile" toml:"profile"`
	Address        string `json:"address" yaml:"address" toml:"address"`
	MaxBodyBytes   int64  `json:"max_body_bytes" yaml:"max_body_bytes" toml:"max_body_bytes"`
	LogLevel       string `json:"log_level" yaml:"log_level" toml:"log_level"`
	DatabaseDriver string `json:"database_driver" yaml:"database_driver" toml:"database_driver"`
	DatabaseDSN    string `json:"database_dsn" yaml:"database_dsn" toml:"database_dsn"`
}

// ResolveOptions controls config resolution precedence.
type ResolveOptions struct {
	Profile   string
	FilePath  string
	EnvPrefix string
}

type partialConfig struct {
	Profile        *string `json:"profile" yaml:"profile" toml:"profile"`
	Address        *string `json:"address" yaml:"address" toml:"address"`
	MaxBodyBytes   *int64  `json:"max_body_bytes" yaml:"max_body_bytes" toml:"max_body_bytes"`
	LogLevel       *string `json:"log_level" yaml:"log_level" toml:"log_level"`
	DatabaseDriver *string `json:"database_driver" yaml:"database_driver" toml:"database_driver"`
	DatabaseDSN    *string `json:"database_dsn" yaml:"database_dsn" toml:"database_dsn"`
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

// KnownProfiles returns supported profile names.
func KnownProfiles() []string {
	return []string{"dev", "prod", "test"}
}

// ProfileDefaults returns the default config for a named profile.
func ProfileDefaults(profile string) (Config, error) {
	profile = strings.ToLower(strings.TrimSpace(profile))
	if profile == "" {
		profile = "dev"
	}

	cfg := Default()
	switch profile {
	case "dev":
		cfg.Profile = "dev"
		cfg.LogLevel = "debug"
	case "test":
		cfg.Profile = "test"
		cfg.Address = ":0"
		cfg.LogLevel = "warn"
		cfg.DatabaseDriver = "sqlite"
		cfg.DatabaseDSN = "file::memory:?cache=shared"
	case "prod":
		cfg.Profile = "prod"
		cfg.LogLevel = "warn"
	default:
		return Config{}, fmt.Errorf("unknown profile %q (supported: %s)", profile, strings.Join(KnownProfiles(), ", "))
	}

	return cfg, nil
}

// LoadFile loads configuration from a file (JSON/YAML/TOML by extension)
// and merges it onto the selected profile defaults.
func LoadFile(path string) (Config, error) {
	override, err := loadFilePartial(path)
	if err != nil {
		return Config{}, err
	}

	profile, err := effectiveProfile("", override.Profile)
	if err != nil {
		return Config{}, err
	}
	cfg, _ := ProfileDefaults(profile)
	cfg = override.apply(cfg)
	cfg.Profile = profile
	return cfg, cfg.Validate()
}

// LoadEnv loads configuration from environment variables.
// Prefix is optional; when provided, variables are looked up as PREFIX_<KEY>.
// Values are merged onto the selected profile defaults.
func LoadEnv(prefix string) (Config, error) {
	override, err := loadEnvPartial(prefix)
	if err != nil {
		return Config{}, err
	}

	profile, err := effectiveProfile("", override.Profile)
	if err != nil {
		return Config{}, err
	}
	cfg, _ := ProfileDefaults(profile)
	cfg = override.apply(cfg)
	cfg.Profile = profile
	return cfg, cfg.Validate()
}

// Resolve loads configuration using strict precedence:
// defaults < profile defaults < file < env
func Resolve(opts ResolveOptions) (Config, error) {
	base := Default()

	fileOverride := partialConfig{}
	var err error
	if strings.TrimSpace(opts.FilePath) != "" {
		fileOverride, err = loadFilePartial(opts.FilePath)
		if err != nil {
			return Config{}, err
		}
	}

	envOverride, err := loadEnvPartial(opts.EnvPrefix)
	if err != nil {
		return Config{}, err
	}

	profile, err := effectiveProfile(opts.Profile, envOverride.Profile, fileOverride.Profile, stringPtr(base.Profile))
	if err != nil {
		return Config{}, err
	}

	cfg, err := ProfileDefaults(profile)
	if err != nil {
		return Config{}, err
	}

	cfg = fileOverride.apply(cfg)
	cfg = envOverride.apply(cfg)
	// Keep profile value aligned with the profile used to derive defaults.
	cfg.Profile = profile

	return cfg, cfg.Validate()
}

// Merge overlays non-zero/nonnull override values onto base.
// Deprecated for strict precedence use-cases: prefer Resolve().
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

func loadFilePartial(path string) (partialConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return partialConfig{}, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	var cfg partialConfig
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return partialConfig{}, fmt.Errorf("parse json config file: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return partialConfig{}, fmt.Errorf("parse yaml config file: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return partialConfig{}, fmt.Errorf("parse toml config file: %w", err)
		}
	default:
		return partialConfig{}, fmt.Errorf("unsupported config file format %q (supported: .json, .yaml, .yml, .toml)", ext)
	}

	return cfg, nil
}

func loadEnvPartial(prefix string) (partialConfig, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix != "" {
		prefix = strings.ToUpper(strings.TrimSuffix(prefix, "_")) + "_"
	}

	var cfg partialConfig
	if profile := os.Getenv(prefix + envProfile); profile != "" {
		cfg.Profile = stringPtr(profile)
	}
	if address := os.Getenv(prefix + envAddress); address != "" {
		cfg.Address = stringPtr(address)
	}
	if logLevel := os.Getenv(prefix + envLogLevel); logLevel != "" {
		cfg.LogLevel = stringPtr(logLevel)
	}
	if dbDriver := os.Getenv(prefix + envDatabaseDriver); dbDriver != "" {
		cfg.DatabaseDriver = stringPtr(dbDriver)
	}
	if dbDSN := os.Getenv(prefix + envDatabaseDSN); dbDSN != "" {
		cfg.DatabaseDSN = stringPtr(dbDSN)
	}
	if raw := os.Getenv(prefix + envMaxBodyBytes); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return partialConfig{}, fmt.Errorf("invalid %s%s: %w", prefix, envMaxBodyBytes, err)
		}
		cfg.MaxBodyBytes = int64Ptr(value)
	}

	return cfg, nil
}

func (p partialConfig) apply(dst Config) Config {
	if p.Profile != nil {
		dst.Profile = *p.Profile
	}
	if p.Address != nil {
		dst.Address = *p.Address
	}
	if p.MaxBodyBytes != nil {
		dst.MaxBodyBytes = *p.MaxBodyBytes
	}
	if p.LogLevel != nil {
		dst.LogLevel = *p.LogLevel
	}
	if p.DatabaseDriver != nil {
		dst.DatabaseDriver = *p.DatabaseDriver
	}
	if p.DatabaseDSN != nil {
		dst.DatabaseDSN = *p.DatabaseDSN
	}
	return dst
}

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func effectiveProfile(explicit string, candidates ...*string) (string, error) {
	if normalized := normalizeProfile(explicit); normalized != "" {
		_, err := ProfileDefaults(normalized)
		if err != nil {
			return "", err
		}
		return normalized, nil
	}

	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if normalized := normalizeProfile(*candidate); normalized != "" {
			_, err := ProfileDefaults(normalized)
			if err != nil {
				return "", err
			}
			return normalized, nil
		}
	}

	def := Default().Profile
	return normalizeProfile(def), nil
}

func normalizeProfile(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
