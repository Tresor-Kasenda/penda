package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
	"penda/framework/middleware"
)

func TestTracingMiddlewareRecordsSpanAndExtractsParent(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	server := fwapp.New()
	server.Use(middleware.RequestID())
	server.Use(Tracing(TracingConfig{
		TracerProvider: tp,
		Propagator:     propagation.TraceContext{},
		TracerName:     "penda-test",
	}))
	server.Get("/hello", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/hello?x=1", nil)
	req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name() != "GET /hello" {
		t.Fatalf("expected span name %q, got %q", "GET /hello", span.Name())
	}
	if !span.Parent().IsValid() {
		t.Fatal("expected parent context extracted from traceparent")
	}
	if got := span.Parent().TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected parent trace id: %s", got)
	}

	attrs := attrsToMap(span.Attributes())
	if attrs["http.request.method"].AsString() != "GET" {
		t.Fatalf("expected http.request.method GET, got %q", attrs["http.request.method"].AsString())
	}
	if attrs["url.path"].AsString() != "/hello" {
		t.Fatalf("expected url.path /hello, got %q", attrs["url.path"].AsString())
	}
	if attrs["http.response.status_code"].AsInt64() != int64(http.StatusOK) {
		t.Fatalf("expected status code attr %d, got %d", http.StatusOK, attrs["http.response.status_code"].AsInt64())
	}
	if attrs["penda.request_id"].AsString() == "" {
		t.Fatal("expected penda.request_id attribute to be set")
	}

	if got := span.SpanContext().TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected child trace id: %s", got)
	}
}

func TestTracingMiddlewareMarks500AsError(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	server := fwapp.New()
	server.Use(Tracing(TracingConfig{
		TracerProvider: tp,
		Propagator:     propagation.TraceContext{},
	}))
	server.Get("/boom", func(c *fwctx.Context) error {
		return fwctx.NewHTTPError(http.StatusInternalServerError, "boom", nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Fatalf("expected span status error, got %v", spans[0].Status().Code)
	}
}

func attrsToMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	out := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, kv := range attrs {
		out[kv.Key] = kv.Value
	}
	return out
}
