package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

func TestSessionMiddlewareSaveAndLoad(t *testing.T) {
	store := MustNewStore([]byte("0123456789abcdef0123456789abcdef"), Config{})

	server := fwapp.New()
	server.Use(Middleware(store))
	server.Get("/set", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		sess.Set("user_id", "42")
		if err := sess.Save(c); err != nil {
			return err
		}
		return c.Text(http.StatusOK, "saved")
	})
	server.Get("/get", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		value, ok := sess.Get("user_id")
		if !ok {
			return c.Text(http.StatusNotFound, "missing")
		}
		return c.Text(http.StatusOK, value)
	})

	reqSet := httptest.NewRequest(http.MethodGet, "/set", nil)
	rrSet := httptest.NewRecorder()
	server.ServeHTTP(rrSet, reqSet)
	if rrSet.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rrSet.Code)
	}
	cookies := rrSet.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/get", nil)
	reqGet.AddCookie(cookies[0])
	rrGet := httptest.NewRecorder()
	server.ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rrGet.Code)
	}
	if strings.TrimSpace(rrGet.Body.String()) != "42" {
		t.Fatalf("unexpected body: %q", rrGet.Body.String())
	}
}

func TestSessionMiddlewareRejectsTamperedCookie(t *testing.T) {
	store := MustNewStore([]byte("0123456789abcdef0123456789abcdef"), Config{})

	server := fwapp.New()
	server.Use(Middleware(store))
	server.Get("/me", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{
		Name:  "penda_session",
		Value: "tampered.payload",
	})
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestSessionDestroy(t *testing.T) {
	store := MustNewStore([]byte("0123456789abcdef0123456789abcdef"), Config{})
	server := fwapp.New()
	server.Use(Middleware(store))
	server.Get("/logout", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		sess.Destroy(c)
		return c.Text(http.StatusOK, "bye")
	})

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie deletion")
	}
	if cookies[0].MaxAge >= 0 {
		t.Fatalf("expected expired cookie MaxAge < 0, got %d", cookies[0].MaxAge)
	}
}
