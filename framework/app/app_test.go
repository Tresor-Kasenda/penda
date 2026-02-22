package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fwconfig "penda/framework/config"
	fwctx "penda/framework/context"
)

func TestGetRoute(t *testing.T) {
	server := New()
	server.Get("/health", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != "ok" {
		t.Fatalf("expected body 'ok', got %q", body)
	}
}

func TestNotFound(t *testing.T) {
	server := New()

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	server := New()
	server.Get("/health", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})
	server.Post("/health", func(c *fwctx.Context) error {
		return c.Text(http.StatusCreated, "created")
	})

	req := httptest.NewRequest(http.MethodDelete, "/health", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	allow := rr.Header().Get("Allow")
	if allow != "GET, POST" {
		t.Fatalf("expected Allow header %q, got %q", "GET, POST", allow)
	}
}

func TestPathParamsAndWildcard(t *testing.T) {
	server := New()
	server.Get("/users/:id/files/*path", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, c.Param("id")+":"+c.Param("path"))
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42/files/images/logo.png", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != "42:images/logo.png" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestMiddlewareExecutionOrderGlobalGroupRoute(t *testing.T) {
	server := New()

	order := make([]string, 0)
	mw := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(c *fwctx.Context) error {
				order = append(order, name+":before")
				err := next(c)
				order = append(order, name+":after")
				return err
			}
		}
	}

	server.Use(mw("global"))
	api := server.Group("/api", mw("group"))
	api.GetWith("/users/:id", func(c *fwctx.Context) error {
		order = append(order, "handler")
		return c.Text(http.StatusOK, c.Param("id"))
	}, mw("route"))

	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	got := strings.Join(order, ",")
	want := "global:before,group:before,route:before,handler,route:after,group:after,global:after"
	if got != want {
		t.Fatalf("unexpected middleware order\nwant: %s\ngot:  %s", want, got)
	}
}

func TestMiddlewareShortCircuitSkipsHandler(t *testing.T) {
	server := New()

	called := false
	server.Use(func(next Handler) Handler {
		return func(c *fwctx.Context) error {
			return c.Text(http.StatusUnauthorized, "blocked")
		}
	})
	server.Get("/protected", func(c *fwctx.Context) error {
		called = true
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if called {
		t.Fatal("expected handler not to be called")
	}
}

func TestGroupPrefixWithoutLeadingSlash(t *testing.T) {
	server := New()
	api := server.Group("api")
	api.Get("health", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestBindJSONErrorReturnsBadRequest(t *testing.T) {
	server := New()
	server.Post("/payload", func(c *fwctx.Context) error {
		var body struct {
			Name string `json:"name" validate:"required"`
		}
		if err := c.BindJSON(&body); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, body)
	})

	req := httptest.NewRequest(http.MethodPost, "/payload", strings.NewReader(`{"name":`))
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMaxBodyBytesReturnsBadRequest(t *testing.T) {
	server := New()
	server.SetMaxBodyBytes(8)
	server.Post("/payload", func(c *fwctx.Context) error {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.BindJSON(&body); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, body)
	})

	req := httptest.NewRequest(http.MethodPost, "/payload", strings.NewReader(`{"name":"value"}`))
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandlerErrorBeforeWritingResponseReturns500(t *testing.T) {
	server := New()
	server.Get("/err", func(c *fwctx.Context) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "boom") {
		t.Fatalf("expected error body to contain %q, got %q", "boom", rr.Body.String())
	}
}

func TestHandlerErrorAfterWritingResponseDoesNotOverrideStatus(t *testing.T) {
	server := New()
	server.Get("/partial", func(c *fwctx.Context) error {
		if err := c.Text(http.StatusAccepted, "accepted"); err != nil {
			return err
		}
		return errors.New("late error")
	})

	req := httptest.NewRequest(http.MethodGet, "/partial", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rr.Code)
	}
}

func TestOnStatusHandlerOverrides404(t *testing.T) {
	server := New()
	server.OnStatus(http.StatusNotFound, func(c *fwctx.Context) error {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "custom-not-found"})
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "custom-not-found") {
		t.Fatalf("expected custom not found body, got %q", rr.Body.String())
	}
}

func TestOnErrorHandler(t *testing.T) {
	server := New()
	server.OnError(func(c *fwctx.Context, err error) error {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream failed"})
	})
	server.Get("/boom", func(c *fwctx.Context) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "upstream failed") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestTemplateRendering(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "index.tmpl")
	if err := os.WriteFile(templatePath, []byte("Hello {{.Name}}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	server := New()
	if err := server.LoadTemplates(filepath.Join(dir, "*.tmpl")); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	server.Get("/page", func(c *fwctx.Context) error {
		return c.Render("index.tmpl", map[string]string{"Name": "Scott"})
	})

	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Hello Scott" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestStaticServing(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.css")
	if err := os.WriteFile(filePath, []byte("body{color:black;}"), 0o644); err != nil {
		t.Fatalf("write static file: %v", err)
	}

	server := New()
	server.Static("/assets", dir)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "color:black") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
	if rr.Header().Get("Cache-Control") == "" {
		t.Fatal("expected Cache-Control header")
	}
}

func TestRoutesListing(t *testing.T) {
	server := New()
	server.Post("/api/users", func(c *fwctx.Context) error {
		c.Status(http.StatusCreated)
		return nil
	})
	server.Get("/health", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	routes := server.Routes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Path != "/api/users" || routes[1].Path != "/health" {
		t.Fatalf("unexpected route listing: %+v", routes)
	}
}

func TestSetConfigUpdatesBodyLimit(t *testing.T) {
	server := New()
	cfg := fwconfig.Default()
	cfg.MaxBodyBytes = 128
	cfg.Address = ":9090"
	if err := server.SetConfig(cfg); err != nil {
		t.Fatalf("set config: %v", err)
	}

	if server.MaxBodyBytes() != 128 {
		t.Fatalf("expected max body bytes 128, got %d", server.MaxBodyBytes())
	}
	if server.Config().Address != ":9090" {
		t.Fatalf("expected address %q, got %q", ":9090", server.Config().Address)
	}
}

func TestLoadConfigResolveOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte("" +
		"profile: prod\n" +
		"max_body_bytes: 0\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("PENDA_ADDRESS", ":7070")

	server := New()
	if err := server.LoadConfig(fwconfig.ResolveOptions{
		FilePath:  path,
		EnvPrefix: "PENDA",
	}); err != nil {
		t.Fatalf("load config: %v", err)
	}

	cfg := server.Config()
	if cfg.Profile != "prod" {
		t.Fatalf("expected profile %q, got %q", "prod", cfg.Profile)
	}
	if cfg.Address != ":7070" {
		t.Fatalf("expected address %q, got %q", ":7070", cfg.Address)
	}
	if cfg.MaxBodyBytes != 0 {
		t.Fatalf("expected max body bytes %d, got %d", 0, cfg.MaxBodyBytes)
	}
}

func BenchmarkServeHTTPExactRoute(b *testing.B) {
	server := New()
	server.Get("/health", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.ServeHTTP(rr, req)
	}
}
