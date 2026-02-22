package app

import (
	"html/template"
	"sync"

	fwconfig "penda/framework/config"
	fwctx "penda/framework/context"
)

// Handler is the function signature for route handlers.
type Handler func(*fwctx.Context) error

// Middleware wraps handlers to implement cross-cutting behavior.
// A middleware can short-circuit by not calling next.
type Middleware func(next Handler) Handler

// ErrorHandler handles uncaught handler errors.
type ErrorHandler func(*fwctx.Context, error) error

// Blueprint is mountable route module.
type Blueprint interface {
	Name() string
	Mount(*App)
}

// RouteInfo describes a registered route.
type RouteInfo struct {
	Method string
	Path   string
}

// App is the main framework object.
type App struct {
	mu                 sync.RWMutex
	routes             []route
	middlewares        []Middleware
	maxBodyBytes       int64
	cfg                fwconfig.Config
	errorHandler       ErrorHandler
	statusHandlers     map[int]Handler
	templateFuncs      template.FuncMap
	templates          *template.Template
	templatePatterns   []string
	templateAutoReload bool
}

// Group scopes routes under a prefix and scoped middleware.
type Group struct {
	app         *App
	prefix      string
	middlewares []Middleware
}

// New creates a new App.
func New() *App {
	cfg := fwconfig.Default()
	return &App{
		routes:         make([]route, 0),
		middlewares:    make([]Middleware, 0),
		maxBodyBytes:   cfg.MaxBodyBytes,
		cfg:            cfg,
		statusHandlers: map[int]Handler{},
		templateFuncs:  template.FuncMap{},
	}
}
