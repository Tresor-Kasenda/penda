package blueprint

import (
	"fmt"
	"html/template"
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

type staticDef struct {
	prefix      string
	dir         string
	middlewares []fwapp.Middleware
}

// Blueprint is a mountable module containing routes and middleware.
type Blueprint struct {
	name             string
	prefix           string
	middlewares      []fwapp.Middleware
	routes           []routeDef
	templatePatterns []string
	templateFuncs    template.FuncMap
	staticMounts     []staticDef
}

// New creates a new blueprint with a prefix.
func New(name, prefix string, middlewares ...fwapp.Middleware) *Blueprint {
	return &Blueprint{
		name:          name,
		prefix:        prefix,
		middlewares:   append([]fwapp.Middleware(nil), middlewares...),
		routes:        make([]routeDef, 0),
		templateFuncs: template.FuncMap{},
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

// SetTemplateFuncs registers template functions that should be available when this
// blueprint loads local templates.
func (b *Blueprint) SetTemplateFuncs(funcs template.FuncMap) {
	for key, value := range funcs {
		b.templateFuncs[key] = value
	}
}

// LoadTemplates registers local template glob patterns to be merged at mount time.
func (b *Blueprint) LoadTemplates(patterns ...string) {
	b.templatePatterns = append(b.templatePatterns, patterns...)
}

// Static registers a local static directory under the blueprint prefix.
func (b *Blueprint) Static(prefix, dir string, middlewares ...fwapp.Middleware) {
	b.StaticWith(prefix, dir, middlewares...)
}

// StaticWith registers a local static directory with route-level middleware.
func (b *Blueprint) StaticWith(prefix, dir string, middlewares ...fwapp.Middleware) {
	b.staticMounts = append(b.staticMounts, staticDef{
		prefix:      prefix,
		dir:         dir,
		middlewares: append([]fwapp.Middleware(nil), middlewares...),
	})
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
	if len(b.templateFuncs) > 0 {
		a.SetTemplateFuncs(b.templateFuncs)
	}
	if len(b.templatePatterns) > 0 {
		if err := a.AppendTemplates(b.templatePatterns...); err != nil {
			panic(fmt.Sprintf("mount blueprint %q templates: %v", b.name, err))
		}
	}

	group := a.Group(b.prefix, b.middlewares...)
	for _, mount := range b.staticMounts {
		group.StaticWith(mount.prefix, mount.dir, mount.middlewares...)
	}
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
