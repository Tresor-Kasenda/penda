package blueprint

import (
	"net/http"
	"net/http/httptest"
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
