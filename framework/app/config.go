package app

import (
	"net/http"
	"strings"

	fwconfig "penda/framework/config"
)

// Config returns a copy of the current app configuration.
func (a *App) Config() fwconfig.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// SetConfig sets app configuration and applies its runtime values.
func (a *App) SetConfig(cfg fwconfig.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	a.mu.Lock()
	a.cfg = cfg
	a.maxBodyBytes = cfg.MaxBodyBytes
	a.mu.Unlock()
	return nil
}

// LoadConfig resolves configuration using profile/file/env precedence and applies it.
func (a *App) LoadConfig(opts fwconfig.ResolveOptions) error {
	cfg, err := fwconfig.Resolve(opts)
	if err != nil {
		return err
	}
	return a.SetConfig(cfg)
}

// LoadConfigFromFile loads config (JSON/YAML/TOML) from file and applies it.
func (a *App) LoadConfigFromFile(path string) error {
	cfg, err := fwconfig.LoadFile(path)
	if err != nil {
		return err
	}
	return a.SetConfig(cfg)
}

// LoadConfigFromEnv loads config from env vars and applies it.
func (a *App) LoadConfigFromEnv(prefix string) error {
	cfg, err := fwconfig.LoadEnv(prefix)
	if err != nil {
		return err
	}
	return a.SetConfig(cfg)
}

// Run starts an HTTP server with the app as handler.
// If addr is empty, config address is used.
func (a *App) Run(addr string) error {
	if strings.TrimSpace(addr) == "" {
		addr = a.Config().Address
	}
	return http.ListenAndServe(addr, a)
}

// Server builds an http.Server with the app as handler.
func (a *App) Server(addr string) *http.Server {
	if strings.TrimSpace(addr) == "" {
		addr = a.Config().Address
	}
	return &http.Server{
		Addr:    addr,
		Handler: a,
	}
}

// SetMaxBodyBytes configures the maximum size allowed for request body parsing.
// Set to 0 or a negative value to disable size limiting.
func (a *App) SetMaxBodyBytes(n int64) {
	a.mu.Lock()
	a.maxBodyBytes = n
	a.cfg.MaxBodyBytes = n
	a.mu.Unlock()
}

// MaxBodyBytes returns the current request body size limit.
func (a *App) MaxBodyBytes() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.maxBodyBytes
}
