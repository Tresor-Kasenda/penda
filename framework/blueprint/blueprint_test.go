package blueprint

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

func TestBlueprintMount(t *testing.T) {
	server := fwapp.New()

	bp := New("users", "/api")
	bp.Get("/users/:id", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, c.Param("id"))
	})

	server.Register(bp)

	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "42" {
		t.Fatalf("expected body %q, got %q", "42", rr.Body.String())
	}
}

func TestBlueprintLocalTemplatesAndStatic(t *testing.T) {
	templateDir := t.TempDir()
	staticDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(templateDir, "page.tmpl"), []byte("Blueprint {{.Name}}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "app.css"), []byte("body{background:#fff;}"), 0o644); err != nil {
		t.Fatalf("write static file: %v", err)
	}

	server := fwapp.New()

	bp := New("web", "/mod")
	bp.LoadTemplates(filepath.Join(templateDir, "*.tmpl"))
	bp.Static("/assets", staticDir)
	bp.Get("/page", func(c *fwctx.Context) error {
		return c.Render("page.tmpl", map[string]string{"Name": "OK"})
	})

	server.Register(bp)

	t.Run("template", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mod/page", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if strings.TrimSpace(rr.Body.String()) != "Blueprint OK" {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})

	t.Run("static", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mod/assets/app.css", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "background") {
			t.Fatalf("unexpected static body: %q", rr.Body.String())
		}
	})
}
