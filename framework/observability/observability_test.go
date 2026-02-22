package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

func TestMetricsMiddlewareAndHandler(t *testing.T) {
	metrics := NewMetrics()
	server := fwapp.New()
	server.Use(metrics.Middleware())
	server.Get("/ok", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})
	server.Get("/metrics", metrics.Handler())

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRR := httptest.NewRecorder()
	server.ServeHTTP(metricsRR, metricsReq)
	if metricsRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, metricsRR.Code)
	}
	body := metricsRR.Body.String()
	if !strings.Contains(body, "penda_requests_total") {
		t.Fatalf("expected metrics body, got %q", body)
	}
}

func TestReadinessHandler(t *testing.T) {
	server := fwapp.New()
	server.Get("/ready", ReadinessHandler(func() error { return nil }))

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
