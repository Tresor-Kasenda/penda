package app

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	fwctx "penda/framework/context"
)

const defaultStaticCacheControl = "public, max-age=3600"

// StaticConfig customizes static file serving behavior.
type StaticConfig struct {
	CacheControl string
	NoCache      bool
	EnableETag   bool
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
	// Manually setting templates disables pattern-based auto-reload source until templates are loaded again.
	a.templatePatterns = nil
	a.mu.Unlock()
}

// SetTemplateAutoReload enables or disables template reparsing on each render call.
// Intended for development mode only.
func (a *App) SetTemplateAutoReload(enabled bool) {
	a.mu.Lock()
	a.templateAutoReload = enabled
	a.mu.Unlock()
}

// TemplateAutoReload reports whether template auto-reload is enabled.
func (a *App) TemplateAutoReload() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.templateAutoReload
}

// LoadTemplates parses template files from one or more glob patterns.
func (a *App) LoadTemplates(patterns ...string) error {
	tmpl, funcs, normalizedPatterns, err := a.parseTemplates(nil, patterns...)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.templates = tmpl
	a.templateFuncs = funcs
	a.templatePatterns = normalizedPatterns
	a.mu.Unlock()
	return nil
}

// AppendTemplates parses template files and merges them into the current template set.
// It preserves previously loaded templates and adds/overrides template definitions by name.
func (a *App) AppendTemplates(patterns ...string) error {
	a.mu.RLock()
	existingPatterns := append([]string(nil), a.templatePatterns...)
	current := a.templates
	a.mu.RUnlock()

	tmpl, funcs, normalizedPatterns, err := a.parseTemplates(current, patterns...)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.templates = tmpl
	a.templateFuncs = funcs
	a.templatePatterns = append(existingPatterns, normalizedPatterns...)
	a.mu.Unlock()
	return nil
}

// Static serves static files from dir under URL prefix.
func (a *App) Static(prefix, dir string) {
	a.StaticWith(prefix, dir)
}

// StaticWith serves static files from dir with route-level middleware.
func (a *App) StaticWith(prefix, dir string, middlewares ...Middleware) {
	a.StaticWithConfig(prefix, dir, StaticConfig{}, middlewares...)
}

// StaticWithConfig serves static files from dir with explicit static serving options.
func (a *App) StaticWithConfig(prefix, dir string, config StaticConfig, middlewares ...Middleware) {
	validateMiddlewares(middlewares)
	config = normalizeStaticConfig(config)

	mount := normalizeGroupPrefix(prefix)
	handler := func(c *fwctx.Context) error {
		if _, err := os.Stat(dir); err != nil {
			return fwctx.NewHTTPError(http.StatusInternalServerError, "invalid static directory", err)
		}

		fullPath, err := resolveStaticPath(dir, c.Param("filepath"))
		if err != nil {
			return fwctx.NewHTTPError(http.StatusNotFound, "file not found", err)
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fwctx.NewHTTPError(http.StatusNotFound, "file not found", err)
			}
			return fwctx.NewHTTPError(http.StatusInternalServerError, "failed to access static file", err)
		}

		if config.NoCache {
			c.SetHeader("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			c.SetHeader("Cache-Control", config.CacheControl)
		}

		if config.EnableETag && info.Mode().IsRegular() {
			etag := staticETag(info)
			c.SetHeader("ETag", etag)
			if ifNoneMatchMatches(c.Header("If-None-Match"), etag) {
				c.Status(http.StatusNotModified)
				return nil
			}
		}

		http.ServeFile(c.Writer, c.Request, fullPath)
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
	g.StaticWithConfig(prefix, dir, StaticConfig{}, middlewares...)
}

// StaticWithConfig serves static files for this group with route middleware and static config.
func (g *Group) StaticWithConfig(prefix, dir string, config StaticConfig, middlewares ...Middleware) {
	validateMiddlewares(middlewares)

	combined := append([]Middleware(nil), g.middlewares...)
	combined = append(combined, middlewares...)
	g.app.StaticWithConfig(joinPaths(g.prefix, prefix), dir, config, combined...)
}

func (a *App) renderTemplate(w http.ResponseWriter, r *http.Request, code int, name string, data any) error {
	templates, err := a.templatesForRender()
	if err != nil {
		return err
	}

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

func (a *App) templatesForRender() (*template.Template, error) {
	a.mu.RLock()
	autoReload := a.templateAutoReload
	patterns := append([]string(nil), a.templatePatterns...)
	current := a.templates
	a.mu.RUnlock()

	if !autoReload || len(patterns) == 0 {
		return current, nil
	}

	tmpl, funcs, _, err := a.parseTemplates(nil, patterns...)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.templates = tmpl
	a.templateFuncs = funcs
	a.mu.Unlock()
	return tmpl, nil
}

func (a *App) parseTemplates(base *template.Template, patterns ...string) (*template.Template, template.FuncMap, []string, error) {
	if len(patterns) == 0 {
		return nil, nil, nil, errors.New("at least one template pattern is required")
	}

	funcs := template.FuncMap{}
	a.mu.RLock()
	for key, value := range a.templateFuncs {
		funcs[key] = value
	}
	a.mu.RUnlock()

	var tmpl *template.Template
	if base != nil {
		cloned, err := base.Clone()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("clone templates: %w", err)
		}
		tmpl = cloned.Funcs(funcs)
	} else {
		tmpl = template.New("").Funcs(funcs)
	}

	normalizedPatterns := make([]string, 0, len(patterns))
	found := false
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		normalizedPatterns = append(normalizedPatterns, pattern)

		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("invalid template pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			continue
		}

		found = true
		if _, err := tmpl.ParseFiles(matches...); err != nil {
			return nil, nil, nil, fmt.Errorf("parse templates for pattern %q: %w", pattern, err)
		}
	}

	if !found {
		return nil, nil, nil, errors.New("no template files matched the provided patterns")
	}

	return tmpl, funcs, normalizedPatterns, nil
}

func normalizeStaticConfig(config StaticConfig) StaticConfig {
	if strings.TrimSpace(config.CacheControl) == "" {
		config.CacheControl = defaultStaticCacheControl
	}
	if !config.EnableETag {
		// Default to ETag enabled unless explicitly disabled through zero value + NoCache use.
		config.EnableETag = true
	}
	return config
}

func resolveStaticPath(rootDir, requested string) (string, error) {
	if strings.ContainsRune(requested, '\x00') {
		return "", errors.New("invalid static path")
	}

	cleaned := path.Clean("/" + requested)
	relative := strings.TrimPrefix(cleaned, "/")
	if relative == "." {
		relative = ""
	}

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(rootAbs, filepath.FromSlash(relative))
	fullAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(rootAbs, fullAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal is not allowed")
	}

	return fullAbs, nil
}

func staticETag(info os.FileInfo) string {
	return fmt.Sprintf(`W/"%x-%x"`, info.Size(), info.ModTime().UnixNano())
}

func ifNoneMatchMatches(headerValue, etag string) bool {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return false
	}
	if headerValue == "*" {
		return true
	}
	for _, part := range strings.Split(headerValue, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}
