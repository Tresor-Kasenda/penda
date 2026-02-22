package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

func TestRecoveryMiddlewareHandlesPanic(t *testing.T) {
	server := fwapp.New()
	server.Use(Recovery())
	server.Get("/panic", func(c *fwctx.Context) error {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestRequestIDMiddlewareSetsHeaderAndLocal(t *testing.T) {
	server := fwapp.New()
	server.Use(RequestID())
	server.Get("/id", func(c *fwctx.Context) error {
		value, ok := c.Get("request_id")
		if !ok {
			t.Fatal("request_id missing from context locals")
		}

		requestID, ok := value.(string)
		if !ok {
			t.Fatalf("expected request_id to be string, got %T", value)
		}
		return c.Text(http.StatusOK, requestID)
	})

	req := httptest.NewRequest(http.MethodGet, "/id", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	bodyID := strings.TrimSpace(rr.Body.String())
	headerID := rr.Header().Get("X-Request-ID")
	if bodyID == "" {
		t.Fatal("expected non-empty request ID in response body")
	}
	if headerID != bodyID {
		t.Fatalf("expected X-Request-ID %q, got %q", bodyID, headerID)
	}
}

func TestTimeoutMiddlewareReturnsGatewayTimeout(t *testing.T) {
	server := fwapp.New()
	server.Use(Timeout(5 * time.Millisecond))
	server.Get("/slow", func(c *fwctx.Context) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status %d, got %d", http.StatusGatewayTimeout, rr.Code)
	}
}

func TestCORSMiddlewareHandlesPreflight(t *testing.T) {
	server := fwapp.New()
	server.Use(CORS(CORSConfig{}))
	server.Get("/ping", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("expected Access-Control-Allow-Origin header")
	}
}

func TestLoggerMiddlewareWritesLogLine(t *testing.T) {
	var buffer bytes.Buffer
	logger := log.New(&buffer, "", 0)

	server := fwapp.New()
	server.Use(Logger(logger))
	server.Get("/log", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	logLine := buffer.String()
	if !strings.Contains(logLine, "GET /log") {
		t.Fatalf("expected log line to include request path, got %q", logLine)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	server := fwapp.New()
	server.Use(SecurityHeaders(SecurityHeadersConfig{}))
	server.Get("/secure", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options header, got %q", rr.Header().Get("X-Content-Type-Options"))
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	server := fwapp.New()
	server.Use(RateLimit(RateLimitConfig{
		Requests: 1,
		Window:   time.Minute,
		KeyFunc: func(c *fwctx.Context) string {
			return "test-key"
		},
	}))
	server.Get("/limited", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req1 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	rr1 := httptest.NewRecorder()
	server.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	rr2 := httptest.NewRecorder()
	server.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	server := fwapp.New()
	server.Use(CSRF(CSRFConfig{}))
	server.Post("/submit", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("_csrf=token123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "token123"})
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCSRFMiddlewareRejectsInvalidToken(t *testing.T) {
	server := fwapp.New()
	server.Use(CSRF(CSRFConfig{}))
	server.Post("/submit", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("_csrf=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "token123"})
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}
