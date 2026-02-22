package blueprint

import (
	"net/http"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

type routeDef struct {
	method      string
	path        string
	handler     fwapp.Handler
	middlewares []fwapp.Middleware
}

// Blueprint is a mountable module containing routes and middleware.
type Blueprint struct {
	name        string
	prefix      string
	middlewares []fwapp.Middleware
	routes      []routeDef
}

// New creates a new blueprint with a prefix.
func New(name, prefix string, middlewares ...fwapp.Middleware) *Blueprint {
	return &Blueprint{
		name:        name,
		prefix:      prefix,
		middlewares: append([]fwapp.Middleware(nil), middlewares...),
		routes:      make([]routeDef, 0),
	}
}

// Name returns blueprint name.
func (b *Blueprint) Name() string {
	return b.name
}

// Use adds middleware to the blueprint.
func (b *Blueprint) Use(middlewares ...fwapp.Middleware) {
	b.middlewares = append(b.middlewares, middlewares...)
}

// Handle registers a route in the blueprint.
func (b *Blueprint) Handle(method, path string, handler fwapp.Handler, middlewares ...fwapp.Middleware) {
	b.routes = append(b.routes, routeDef{
		method:      method,
		path:        path,
		handler:     handler,
		middlewares: append([]fwapp.Middleware(nil), middlewares...),
	})
}

// Get registers a GET route.
func (b *Blueprint) Get(path string, handler fwapp.Handler, middlewares ...fwapp.Middleware) {
	b.Handle(http.MethodGet, path, handler, middlewares...)
}

// Post registers a POST route.
func (b *Blueprint) Post(path string, handler fwapp.Handler, middlewares ...fwapp.Middleware) {
	b.Handle(http.MethodPost, path, handler, middlewares...)
}

// Put registers a PUT route.
func (b *Blueprint) Put(path string, handler fwapp.Handler, middlewares ...fwapp.Middleware) {
	b.Handle(http.MethodPut, path, handler, middlewares...)
}

// Patch registers a PATCH route.
func (b *Blueprint) Patch(path string, handler fwapp.Handler, middlewares ...fwapp.Middleware) {
	b.Handle(http.MethodPatch, path, handler, middlewares...)
}

// Delete registers a DELETE route.
func (b *Blueprint) Delete(path string, handler fwapp.Handler, middlewares ...fwapp.Middleware) {
	b.Handle(http.MethodDelete, path, handler, middlewares...)
}

// Mount mounts blueprint routes into app.
func (b *Blueprint) Mount(a *fwapp.App) {
	group := a.Group(b.prefix, b.middlewares...)
	for _, route := range b.routes {
		group.HandleWith(route.method, route.path, route.handler, route.middlewares...)
	}
}

// HealthBlueprint returns a simple health blueprint used by examples/tests.
func HealthBlueprint(prefix string) *Blueprint {
	bp := New("health", prefix)
	bp.Get("/health", func(c *fwctx.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	return bp
}
