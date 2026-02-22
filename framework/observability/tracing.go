package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

// TracingConfig configures OpenTelemetry HTTP request tracing.
type TracingConfig struct {
	TracerProvider oteltrace.TracerProvider
	Propagator     propagation.TextMapPropagator
	TracerName     string
	SpanNameFunc   func(*fwctx.Context) string
}

// OTLPHTTPTracerProviderConfig configures an OTLP/HTTP exporter-backed tracer provider.
type OTLPHTTPTracerProviderConfig struct {
	Endpoint       string
	Insecure       bool
	Headers        map[string]string
	ServiceName    string
	ServiceVersion string
	Environment    string
	Sampler        sdktrace.Sampler
}

// Tracing returns an OpenTelemetry tracing middleware for incoming HTTP requests.
func Tracing(config TracingConfig) fwapp.Middleware {
	cfg := normalizeTracingConfig(config)
	tracer := cfg.TracerProvider.Tracer(cfg.TracerName)

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			ctx := cfg.Propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
			c.Request = c.Request.WithContext(ctx)

			spanName := cfg.SpanNameFunc(c)
			if strings.TrimSpace(spanName) == "" {
				spanName = c.Request.Method + " " + c.Request.URL.Path
			}

			start := time.Now()
			ctx, span := tracer.Start(ctx, spanName, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
			defer span.End()
			c.Request = c.Request.WithContext(ctx)

			span.SetAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("url.path", c.Request.URL.Path),
				attribute.String("url.query", c.Request.URL.RawQuery),
				attribute.String("user_agent.original", c.Request.UserAgent()),
			)
			if host := strings.TrimSpace(c.Request.Host); host != "" {
				span.SetAttributes(attribute.String("server.address", host))
			}
			if requestID, ok := c.Get("request_id"); ok {
				if value, ok := requestID.(string); ok && strings.TrimSpace(value) != "" {
					span.SetAttributes(attribute.String("penda.request_id", value))
				}
			}

			err := next(c)

			status := c.StatusCode()
			if err != nil {
				if errStatus := statusCodeFromError(err); errStatus != 0 {
					status = errStatus
				}
			}
			if status == 0 {
				status = statusCodeFromError(err)
			}
			if status == 0 {
				status = http.StatusOK
			}

			span.SetAttributes(
				attribute.Int("http.response.status_code", status),
				attribute.Int64("penda.request.duration_ms", time.Since(start).Milliseconds()),
			)

			if err != nil {
				span.RecordError(err)
			}
			if status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(status))
			} else {
				span.SetStatus(codes.Ok, "")
			}

			return err
		}
	}
}

// NewOTLPHTTPTracerProvider creates a tracer provider exporting spans via OTLP/HTTP.
func NewOTLPHTTPTracerProvider(ctx context.Context, config OTLPHTTPTracerProviderConfig) (*sdktrace.TracerProvider, error) {
	cfg := normalizeOTLPHTTPTracerProviderConfig(config)

	options := []otlptracehttp.Option{}
	if cfg.Endpoint != "" {
		options = append(options, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		options = append(options, otlptracehttp.WithHeaders(cfg.Headers))
	}

	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create otlp http trace exporter: %w", err)
	}

	resourceAttrs := []attribute.KeyValue{
		attribute.String("service.name", cfg.ServiceName),
	}
	if strings.TrimSpace(cfg.ServiceVersion) != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("service.version", cfg.ServiceVersion))
	}
	if strings.TrimSpace(cfg.Environment) != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("deployment.environment", cfg.Environment))
	}

	res, err := sdkresource.New(ctx, sdkresource.WithAttributes(resourceAttrs...))
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(cfg.Sampler),
	), nil
}

func normalizeTracingConfig(config TracingConfig) TracingConfig {
	if config.TracerProvider == nil {
		config.TracerProvider = otel.GetTracerProvider()
	}
	if config.Propagator == nil {
		config.Propagator = otel.GetTextMapPropagator()
	}
	if strings.TrimSpace(config.TracerName) == "" {
		config.TracerName = "penda/http"
	}
	if config.SpanNameFunc == nil {
		config.SpanNameFunc = func(c *fwctx.Context) string {
			return c.Request.Method + " " + c.Request.URL.Path
		}
	}
	return config
}

func normalizeOTLPHTTPTracerProviderConfig(config OTLPHTTPTracerProviderConfig) OTLPHTTPTracerProviderConfig {
	if strings.TrimSpace(config.ServiceName) == "" {
		config.ServiceName = "penda-app"
	}
	if config.Sampler == nil {
		config.Sampler = sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
	if config.Headers == nil {
		config.Headers = map[string]string{}
	}
	return config
}
