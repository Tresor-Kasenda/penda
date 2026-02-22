package observability

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

// Metrics holds in-memory request metrics.
type Metrics struct {
	inFlight atomic.Int64

	mu                sync.Mutex
	totalRequests     uint64
	durationTotalMS   float64
	byMethodAndStatus map[string]uint64
}

// NewMetrics creates a metrics recorder.
func NewMetrics() *Metrics {
	return &Metrics{
		byMethodAndStatus: map[string]uint64{},
	}
}

// Middleware records basic request metrics.
func (m *Metrics) Middleware() fwapp.Middleware {
	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			start := time.Now()
			m.inFlight.Add(1)
			defer m.inFlight.Add(-1)

			err := next(c)

			status := c.StatusCode()
			if status == 0 {
				status = statusCodeFromError(err)
			}
			if status == 0 {
				status = http.StatusOK
			}

			key := metricKey(c.Request.Method, status)
			elapsed := float64(time.Since(start).Milliseconds())

			m.mu.Lock()
			m.totalRequests++
			m.durationTotalMS += elapsed
			m.byMethodAndStatus[key]++
			m.mu.Unlock()
			return err
		}
	}
}

// Handler returns metrics in Prometheus text format.
func (m *Metrics) Handler() fwapp.Handler {
	return func(c *fwctx.Context) error {
		payload := m.snapshot()
		c.SetHeader("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		return c.Text(http.StatusOK, payload)
	}
}

// HealthHandler returns a simple health endpoint.
func HealthHandler() fwapp.Handler {
	return func(c *fwctx.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ReadinessHandler executes a check and returns readiness status.
func ReadinessHandler(check func() error) fwapp.Handler {
	return func(c *fwctx.Context) error {
		if check != nil {
			if err := check(); err != nil {
				return fwctx.NewHTTPError(http.StatusServiceUnavailable, "not ready", err)
			}
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	}
}

func (m *Metrics) snapshot() string {
	m.mu.Lock()
	total := m.totalRequests
	durationTotalMS := m.durationTotalMS
	byMethodAndStatus := map[string]uint64{}
	for key, value := range m.byMethodAndStatus {
		byMethodAndStatus[key] = value
	}
	m.mu.Unlock()

	var builder strings.Builder
	builder.WriteString("# TYPE penda_requests_total counter\n")
	builder.WriteString(fmt.Sprintf("penda_requests_total %d\n", total))
	builder.WriteString("# TYPE penda_requests_in_flight gauge\n")
	builder.WriteString(fmt.Sprintf("penda_requests_in_flight %d\n", m.inFlight.Load()))
	builder.WriteString("# TYPE penda_request_duration_ms_total counter\n")
	builder.WriteString(fmt.Sprintf("penda_request_duration_ms_total %.0f\n", durationTotalMS))
	builder.WriteString("# TYPE penda_requests_by_method_status_total counter\n")

	for key, value := range byMethodAndStatus {
		method, status := splitMetricKey(key)
		builder.WriteString(
			fmt.Sprintf(
				"penda_requests_by_method_status_total{method=%q,status=%q} %d\n",
				method,
				status,
				value,
			),
		)
	}

	return builder.String()
}

func metricKey(method string, statusCode int) string {
	return method + "|" + strconv.Itoa(statusCode)
}

func splitMetricKey(key string) (string, string) {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		return key, "0"
	}
	return parts[0], parts[1]
}

func statusCodeFromError(err error) int {
	type statusCoder interface {
		StatusCode() int
	}

	var target statusCoder
	if errors.As(err, &target) {
		return target.StatusCode()
	}
	return 0
}
