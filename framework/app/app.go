package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	fwconfig "penda/framework/config"
	fwctx "penda/framework/context"
)

const defaultStaticCacheControl = "public, max-age=3600"

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
	mu             sync.RWMutex
	routes         []route
	middlewares    []Middleware
	maxBodyBytes   int64
	cfg            fwconfig.Config
	errorHandler   ErrorHandler
	statusHandlers map[int]Handler
	templateFuncs  template.FuncMap
	templates      *template.Template
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

// LoadConfigFromFile loads JSON config from file and applies it.
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

// Use registers global middleware in declaration order.
func (a *App) Use(middlewares ...Middleware) {
	validateMiddlewares(middlewares)

	a.mu.Lock()
	a.middlewares = append(a.middlewares, middlewares...)
	a.mu.Unlock()
}

// OnError registers a global error handler.
func (a *App) OnError(handler ErrorHandler) {
	if handler == nil {
		panic("error handler cannot be nil")
	}

	a.mu.Lock()
	a.errorHandler = handler
	a.mu.Unlock()
}

// OnStatus registers a status-specific fallback handler.
func (a *App) OnStatus(code int, handler Handler) {
	if handler == nil {
		panic("status handler cannot be nil")
	}
	if code < 100 || code > 599 {
		panic("invalid HTTP status code")
	}

	a.mu.Lock()
	a.statusHandlers[code] = handler
	a.mu.Unlock()
}

// Group creates a route group with a shared path prefix and middleware.
func (a *App) Group(prefix string, middlewares ...Middleware) *Group {
	validateMiddlewares(middlewares)

	return &Group{
		app:         a,
		prefix:      normalizeGroupPrefix(prefix),
		middlewares: append([]Middleware(nil), middlewares...),
	}
}

// Use registers middleware in declaration order for this group.
func (g *Group) Use(middlewares ...Middleware) {
	validateMiddlewares(middlewares)
	g.middlewares = append(g.middlewares, middlewares...)
}

// Group creates a nested group inheriting parent middleware.
func (g *Group) Group(prefix string, middlewares ...Middleware) *Group {
	validateMiddlewares(middlewares)

	combined := append([]Middleware(nil), g.middlewares...)
	combined = append(combined, middlewares...)

	return &Group{
		app:         g.app,
		prefix:      joinPaths(g.prefix, prefix),
		middlewares: combined,
	}
}

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

// SetTemplateFuncs registers template helpers for future template parsing.
func (a *App) SetTemplateFuncs(funcs template.FuncMap) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for key, value := range funcs {
		a.templateFuncs[key] = value
	}
}

// SetTemplates sets the template set to use for render calls.
func (a *App) SetTemplates(templates *template.Template) {
	a.mu.Lock()
	a.templates = templates
	a.mu.Unlock()
}

// LoadTemplates parses template files from one or more glob patterns.
func (a *App) LoadTemplates(patterns ...string) error {
	if len(patterns) == 0 {
		return errors.New("at least one template pattern is required")
	}

	funcs := template.FuncMap{}
	a.mu.RLock()
	for key, value := range a.templateFuncs {
		funcs[key] = value
	}
	a.mu.RUnlock()

	tmpl := template.New("").Funcs(funcs)
	found := false
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid template pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			continue
		}

		found = true
		if _, err := tmpl.ParseFiles(matches...); err != nil {
			return fmt.Errorf("parse templates for pattern %q: %w", pattern, err)
		}
	}

	if !found {
		return errors.New("no template files matched the provided patterns")
	}

	a.mu.Lock()
	a.templates = tmpl
	a.mu.Unlock()
	return nil
}

// Static serves static files from dir under URL prefix.
func (a *App) Static(prefix, dir string) {
	a.StaticWith(prefix, dir)
}

// StaticWith serves static files from dir with route-level middleware.
func (a *App) StaticWith(prefix, dir string, middlewares ...Middleware) {
	validateMiddlewares(middlewares)

	mount := normalizeGroupPrefix(prefix)
	stripPrefix := mount
	if stripPrefix == "" {
		stripPrefix = "/"
	}

	fileServer := http.StripPrefix(stripPrefix, http.FileServer(http.Dir(dir)))
	handler := func(c *fwctx.Context) error {
		if _, err := os.Stat(dir); err != nil {
			return fwctx.NewHTTPError(http.StatusInternalServerError, "invalid static directory", err)
		}

		c.SetHeader("Cache-Control", defaultStaticCacheControl)
		fileServer.ServeHTTP(c.Writer, c.Request)
		return nil
	}

	pattern := joinPaths(mount, "/*filepath")
	a.GetWith(pattern, handler, middlewares...)
	a.HandleWith(http.MethodHead, pattern, handler, middlewares...)
}

// Static serves static files for this group.
func (g *Group) Static(prefix, dir string) {
	g.StaticWith(prefix, dir)
}

// StaticWith serves static files for this group with route middleware.
func (g *Group) StaticWith(prefix, dir string, middlewares ...Middleware) {
	validateMiddlewares(middlewares)

	combined := append([]Middleware(nil), g.middlewares...)
	combined = append(combined, middlewares...)
	g.app.StaticWith(joinPaths(g.prefix, prefix), dir, combined...)
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

// ServeHTTP dispatches incoming requests to registered routes.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := a.runtimeState()

	writer := &statusWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
	if state.maxBodyBytes > 0 && r.Body != nil {
		r.Body = http.MaxBytesReader(writer, r.Body, state.maxBodyBytes)
	}

	ctx := fwctx.New(writer, r, nil)
	ctx.SetRenderer(a.renderTemplate)

	dispatch := func(c *fwctx.Context) error {
		rt, params, allowed, matched := a.match(c.Request.Method, c.Request.URL.Path)
		if !matched {
			if len(allowed) > 0 {
				c.SetHeader("Allow", strings.Join(allowed, ", "))
				return fwctx.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed", nil)
			}
			return fwctx.NewHTTPError(http.StatusNotFound, "not found", nil)
		}

		c.Params = params
		return chain(rt.handler, rt.middlewares)(c)
	}

	composed := chain(dispatch, state.middlewares)
	if err := composed(ctx); err != nil {
		if writer.wroteHeader {
			return
		}
		a.handleError(ctx, writer, err, state.errorHandler, state.statusHandlers)
	}
}

type runtimeState struct {
	middlewares    []Middleware
	maxBodyBytes   int64
	errorHandler   ErrorHandler
	statusHandlers map[int]Handler
}

func (a *App) runtimeState() runtimeState {
	a.mu.RLock()
	defer a.mu.RUnlock()

	middlewares := append([]Middleware(nil), a.middlewares...)
	statusHandlers := map[int]Handler{}
	for key, handler := range a.statusHandlers {
		statusHandlers[key] = handler
	}

	return runtimeState{
		middlewares:    middlewares,
		maxBodyBytes:   a.maxBodyBytes,
		errorHandler:   a.errorHandler,
		statusHandlers: statusHandlers,
	}
}

func (a *App) handleError(c *fwctx.Context, writer *statusWriter, err error, globalErrorHandler ErrorHandler, statusHandlers map[int]Handler) {
	statusCode := statusCodeFromError(err)

	if handler, ok := statusHandlers[statusCode]; ok && handler != nil {
		if statusErr := handler(c); statusErr == nil {
			return
		} else {
			err = statusErr
			statusCode = statusCodeFromError(err)
		}
	}

	if globalErrorHandler != nil {
		if hErr := globalErrorHandler(c, err); hErr == nil {
			return
		} else {
			err = hErr
			statusCode = statusCodeFromError(err)
		}
	}

	if writer.wroteHeader {
		return
	}

	if acceptsJSON(c.Request) {
		_ = c.JSON(statusCode, map[string]any{
			"error": err.Error(),
		})
		return
	}

	if statusCode == http.StatusNotFound {
		http.NotFound(writer, c.Request)
		return
	}

	http.Error(writer, err.Error(), statusCode)
}

func (a *App) renderTemplate(w http.ResponseWriter, r *http.Request, code int, name string, data any) error {
	a.mu.RLock()
	templates := a.templates
	a.mu.RUnlock()

	if templates == nil {
		return fwctx.NewHTTPError(http.StatusInternalServerError, "templates are not loaded", nil)
	}

	var buffer bytes.Buffer
	if err := templates.ExecuteTemplate(&buffer, name, data); err != nil {
		return fwctx.NewHTTPError(http.StatusInternalServerError, "failed to render template", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if _, err := io.Copy(w, &buffer); err != nil {
		return fwctx.NewHTTPError(http.StatusInternalServerError, "failed to write rendered template", err)
	}
	return nil
}

func acceptsJSON(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(accept, "application/json") || strings.Contains(accept, "text/json") {
		return true
	}
	if accept == "" && strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return true
	}
	return false
}

func (a *App) match(method, path string) (route, map[string]string, []string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	allowedSet := map[string]struct{}{}

	for i := range a.routes {
		rt := &a.routes[i]
		params, ok := matchPath(rt.parts, path)
		if !ok {
			continue
		}

		if rt.method == normalizedMethod {
			copied := *rt
			copied.middlewares = append([]Middleware(nil), rt.middlewares...)
			return copied, params, nil, true
		}

		allowedSet[rt.method] = struct{}{}
	}

	if len(allowedSet) == 0 {
		return route{}, nil, nil, false
	}

	allowed := make([]string, 0, len(allowedSet))
	for methodName := range allowedSet {
		allowed = append(allowed, methodName)
	}
	slices.Sort(allowed)

	return route{}, nil, allowed, false
}

func chain(handler Handler, middlewares []Middleware) Handler {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

func validateMiddlewares(middlewares []Middleware) {
	for i := range middlewares {
		if middlewares[i] == nil {
			panic("middleware cannot be nil")
		}
	}
}

func statusCodeFromError(err error) int {
	type statusCoder interface {
		StatusCode() int
	}

	var target statusCoder
	if errors.As(err, &target) {
		code := target.StatusCode()
		if code >= 100 && code <= 599 {
			return code
		}
	}

	return http.StatusInternalServerError
}

type route struct {
	method      string
	pattern     string
	parts       []segment
	handler     Handler
	middlewares []Middleware
}

type segmentKind int

const (
	segmentStatic segmentKind = iota
	segmentParam
	segmentWildcard
)

type segment struct {
	kind  segmentKind
	value string
}

func parsePattern(path string) ([]segment, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}
	if path[0] != '/' {
		return nil, errors.New("path must start with /")
	}

	parts := splitPath(path)
	segments := make([]segment, 0, len(parts))

	for idx, part := range parts {
		switch {
		case strings.HasPrefix(part, ":"):
			name := strings.TrimPrefix(part, ":")
			if name == "" {
				return nil, errors.New("path param name cannot be empty")
			}
			segments = append(segments, segment{kind: segmentParam, value: name})
		case strings.HasPrefix(part, "*"):
			name := strings.TrimPrefix(part, "*")
			if name == "" {
				return nil, errors.New("wildcard name cannot be empty")
			}
			if idx != len(parts)-1 {
				return nil, errors.New("wildcard must be the last segment")
			}
			segments = append(segments, segment{kind: segmentWildcard, value: name})
		default:
			segments = append(segments, segment{kind: segmentStatic, value: part})
		}
	}

	return segments, nil
}

func matchPath(pattern []segment, path string) (map[string]string, bool) {
	parts := splitPath(path)
	params := map[string]string{}

	pathIdx := 0
	for patternIdx := 0; patternIdx < len(pattern); patternIdx++ {
		seg := pattern[patternIdx]
		if seg.kind == segmentWildcard {
			params[seg.value] = strings.Join(parts[pathIdx:], "/")
			return params, true
		}

		if pathIdx >= len(parts) {
			return nil, false
		}

		part := parts[pathIdx]
		switch seg.kind {
		case segmentStatic:
			if seg.value != part {
				return nil, false
			}
		case segmentParam:
			params[seg.value] = part
		}

		pathIdx++
	}

	if pathIdx != len(parts) {
		return nil, false
	}

	return params, true
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func normalizeGroupPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return ""
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	return strings.TrimRight(prefix, "/")
}

func joinPaths(prefix, path string) string {
	if prefix == "/" {
		prefix = ""
	}
	prefix = strings.TrimRight(prefix, "/")

	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if prefix == "" {
		return path
	}

	return prefix + path
}

type statusWriter struct {
	http.ResponseWriter
	wroteHeader bool
	statusCode  int
}

func (w *statusWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// StatusCode returns the last status code written.
func (w *statusWriter) StatusCode() int {
	return w.statusCode
}

// MarshalJSON serializes routes for diagnostics and tooling.
func (a *App) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Routes())
}
