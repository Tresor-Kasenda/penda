package app

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"

	fwctx "penda/framework/context"
)

const defaultStaticCacheControl = "public, max-age=3600"

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

// AppendTemplates parses template files and merges them into the current template set.
// It preserves previously loaded templates and adds/overrides template definitions by name.
func (a *App) AppendTemplates(patterns ...string) error {
	if len(patterns) == 0 {
		return errors.New("at least one template pattern is required")
	}

	funcs := template.FuncMap{}
	a.mu.RLock()
	for key, value := range a.templateFuncs {
		funcs[key] = value
	}
	current := a.templates
	a.mu.RUnlock()

	var tmpl *template.Template
	if current != nil {
		cloned, err := current.Clone()
		if err != nil {
			return fmt.Errorf("clone templates: %w", err)
		}
		tmpl = cloned.Funcs(funcs)
	} else {
		tmpl = template.New("").Funcs(funcs)
	}

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
