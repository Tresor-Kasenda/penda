package app

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
)

// Register mounts a blueprint module into the app.
func (a *App) Register(bp Blueprint) {
	if bp == nil {
		panic("blueprint cannot be nil")
	}
	bp.Mount(a)
}

// Routes returns all registered routes.
func (a *App) Routes() []RouteInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]RouteInfo, 0, len(a.routes))
	for _, rt := range a.routes {
		out = append(out, RouteInfo{
			Method: rt.method,
			Path:   rt.pattern,
		})
	}

	slices.SortFunc(out, func(left, right RouteInfo) int {
		if left.Path == right.Path {
			return strings.Compare(left.Method, right.Method)
		}
		return strings.Compare(left.Path, right.Path)
	})

	return out
}

// MarshalJSON serializes routes for diagnostics and tooling.
func (a *App) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Routes())
}

// Handle registers a route for an HTTP method and path.
// It panics if method/path is invalid or handler is nil.
func (a *App) Handle(method, path string, handler Handler) {
	a.HandleWith(method, path, handler)
}

// HandleWith registers a route with route-scoped middleware.
func (a *App) HandleWith(method, path string, handler Handler, middlewares ...Middleware) {
	if handler == nil {
		panic("handler cannot be nil")
	}
	validateMiddlewares(middlewares)

	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		panic("method cannot be empty")
	}

	parts, err := parsePattern(path)
	if err != nil {
		panic(err)
	}

	a.mu.Lock()
	a.routes = append(a.routes, route{
		method:      normalizedMethod,
		pattern:     path,
		parts:       parts,
		handler:     handler,
		middlewares: append([]Middleware(nil), middlewares...),
	})
	a.mu.Unlock()
}

// Handle registers a route under the group prefix.
func (g *Group) Handle(method, path string, handler Handler) {
	g.HandleWith(method, path, handler)
}

// HandleWith registers a route under the group prefix with route middleware.
func (g *Group) HandleWith(method, path string, handler Handler, middlewares ...Middleware) {
	validateMiddlewares(middlewares)

	routeMiddlewares := append([]Middleware(nil), g.middlewares...)
	routeMiddlewares = append(routeMiddlewares, middlewares...)
	g.app.HandleWith(method, joinPaths(g.prefix, path), handler, routeMiddlewares...)
}

// Get registers a GET route.
func (a *App) Get(path string, handler Handler) {
	a.Handle(http.MethodGet, path, handler)
}

// GetWith registers a GET route with route middleware.
func (a *App) GetWith(path string, handler Handler, middlewares ...Middleware) {
	a.HandleWith(http.MethodGet, path, handler, middlewares...)
}

// Get registers a GET route under this group.
func (g *Group) Get(path string, handler Handler) {
	g.Handle(http.MethodGet, path, handler)
}

// GetWith registers a GET route under this group with route middleware.
func (g *Group) GetWith(path string, handler Handler, middlewares ...Middleware) {
	g.HandleWith(http.MethodGet, path, handler, middlewares...)
}

// Post registers a POST route.
func (a *App) Post(path string, handler Handler) {
	a.Handle(http.MethodPost, path, handler)
}

// PostWith registers a POST route with route middleware.
func (a *App) PostWith(path string, handler Handler, middlewares ...Middleware) {
	a.HandleWith(http.MethodPost, path, handler, middlewares...)
}

// Post registers a POST route under this group.
func (g *Group) Post(path string, handler Handler) {
	g.Handle(http.MethodPost, path, handler)
}

// PostWith registers a POST route under this group with route middleware.
func (g *Group) PostWith(path string, handler Handler, middlewares ...Middleware) {
	g.HandleWith(http.MethodPost, path, handler, middlewares...)
}

// Put registers a PUT route.
func (a *App) Put(path string, handler Handler) {
	a.Handle(http.MethodPut, path, handler)
}

// PutWith registers a PUT route with route middleware.
func (a *App) PutWith(path string, handler Handler, middlewares ...Middleware) {
	a.HandleWith(http.MethodPut, path, handler, middlewares...)
}

// Put registers a PUT route under this group.
func (g *Group) Put(path string, handler Handler) {
	g.Handle(http.MethodPut, path, handler)
}

// PutWith registers a PUT route under this group with route middleware.
func (g *Group) PutWith(path string, handler Handler, middlewares ...Middleware) {
	g.HandleWith(http.MethodPut, path, handler, middlewares...)
}

// Delete registers a DELETE route.
func (a *App) Delete(path string, handler Handler) {
	a.Handle(http.MethodDelete, path, handler)
}

// DeleteWith registers a DELETE route with route middleware.
func (a *App) DeleteWith(path string, handler Handler, middlewares ...Middleware) {
	a.HandleWith(http.MethodDelete, path, handler, middlewares...)
}

// Delete registers a DELETE route under this group.
func (g *Group) Delete(path string, handler Handler) {
	g.Handle(http.MethodDelete, path, handler)
}

// DeleteWith registers a DELETE route under this group with route middleware.
func (g *Group) DeleteWith(path string, handler Handler, middlewares ...Middleware) {
	g.HandleWith(http.MethodDelete, path, handler, middlewares...)
}

// Patch registers a PATCH route.
func (a *App) Patch(path string, handler Handler) {
	a.Handle(http.MethodPatch, path, handler)
}

// PatchWith registers a PATCH route with route middleware.
func (a *App) PatchWith(path string, handler Handler, middlewares ...Middleware) {
	a.HandleWith(http.MethodPatch, path, handler, middlewares...)
}

// Patch registers a PATCH route under this group.
func (g *Group) Patch(path string, handler Handler) {
	g.Handle(http.MethodPatch, path, handler)
}

// PatchWith registers a PATCH route under this group with route middleware.
func (g *Group) PatchWith(path string, handler Handler, middlewares ...Middleware) {
	g.HandleWith(http.MethodPatch, path, handler, middlewares...)
}
