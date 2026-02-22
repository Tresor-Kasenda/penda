package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

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

func TestRedisRateLimitMiddleware(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	server := fwapp.New()
	server.Use(RedisRateLimit(client, RedisRateLimitConfig{
		Requests: 1,
		Window:   time.Minute,
		KeyFunc: func(c *fwctx.Context) string {
			return "test-key"
		},
		KeyPrefix: "test:rl:",
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
	if rr1.Header().Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("expected X-RateLimit-Limit=1, got %q", rr1.Header().Get("X-RateLimit-Limit"))
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
	if rr2.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("expected X-RateLimit-Remaining=0, got %q", rr2.Header().Get("X-RateLimit-Remaining"))
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

func TestCSRFMiddlewareIssuesTokenOnSafeMethod(t *testing.T) {
	server := fwapp.New()
	server.Use(CSRF(CSRFConfig{}))
	server.Get("/form", func(c *fwctx.Context) error {
		token := CSRFToken(c)
		if strings.TrimSpace(token) == "" {
			t.Fatal("expected csrf token in context")
		}
		return c.Text(http.StatusOK, token)
	})

	req := httptest.NewRequest(http.MethodGet, "/form", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie to be set")
	}
	if strings.TrimSpace(cookies[0].Value) == "" {
		t.Fatal("expected csrf cookie value")
	}
	if strings.TrimSpace(rr.Body.String()) != cookies[0].Value {
		t.Fatalf("expected body token to match cookie token, got body=%q cookie=%q", rr.Body.String(), cookies[0].Value)
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

func TestCSRFMiddlewareRotateOnUnsafe(t *testing.T) {
	server := fwapp.New()
	server.Use(CSRF(CSRFConfig{RotateOnUnsafe: true}))
	server.Post("/submit", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	initialGet := httptest.NewRequest(http.MethodGet, "/missing-safe-route", nil)
	initialRR := httptest.NewRecorder()
	// Route 404 still passes through global middleware and should issue a token.
	server.ServeHTTP(initialRR, initialGet)
	if len(initialRR.Result().Cookies()) == 0 {
		t.Fatal("expected csrf cookie issued on safe request")
	}
	initialCookie := initialRR.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.Header.Set("X-CSRF-Token", initialCookie.Value)
	req.AddCookie(initialCookie)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	nextCookies := rr.Result().Cookies()
	if len(nextCookies) == 0 {
		t.Fatal("expected rotated csrf cookie")
	}
	if nextCookies[0].Value == initialCookie.Value {
		t.Fatal("expected rotated csrf token to differ from original")
	}
}
