package app

import (
	"errors"
	"net/http"
	"strings"

	fwctx "penda/framework/context"
)

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
