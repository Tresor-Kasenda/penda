package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

var requestIDCounter uint64

// CORSConfig holds basic CORS options.
type CORSConfig struct {
	AllowOrigin      string
	AllowMethods     string
	AllowHeaders     string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// SecurityHeadersConfig holds common security header values.
type SecurityHeadersConfig struct {
	ContentSecurityPolicy   string
	ReferrerPolicy          string
	XFrameOptions           string
	XContentTypeOptions     string
	PermissionsPolicy       string
	StrictTransportSecurity string
}

// RateLimitConfig configures in-memory per-key rate limiting.
type RateLimitConfig struct {
	Requests int
	Window   time.Duration
	KeyFunc  func(*fwctx.Context) string
}

// CSRFConfig configures CSRF token validation.
type CSRFConfig struct {
	CookieName string
	HeaderName string
	FormField  string
}

// Recovery catches panics and turns them into a 500 error.
func Recovery() fwapp.Middleware {
	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					err = fwctx.NewHTTPError(
						http.StatusInternalServerError,
						"internal server error",
						fmt.Errorf("panic recovered: %v", rec),
					)
				}
			}()
			return next(c)
		}
	}
}

// Logger logs method/path/status/duration for each request.
func Logger(logger *log.Logger) fwapp.Middleware {
	if logger == nil {
		logger = log.Default()
	}

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			start := time.Now()
			err := next(c)
			status := c.StatusCode()

			type statusCoder interface {
				StatusCode() int
			}
			var target statusCoder
			if errors.As(err, &target) {
				status = target.StatusCode()
			}
			if status == 0 {
				status = http.StatusOK
			}

			logger.Printf("%s %s -> %d (%s)", c.Request.Method, c.Request.URL.Path, status, time.Since(start))
			return err
		}
	}
}

// RequestID sets/propagates X-Request-ID for each request.
func RequestID() fwapp.Middleware {
	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			requestID := strings.TrimSpace(c.Header("X-Request-ID"))
			if requestID == "" {
				requestID = newRequestID()
			}

			c.Set("request_id", requestID)
			c.SetHeader("X-Request-ID", requestID)
			return next(c)
		}
	}
}

// Timeout injects a request context deadline and returns 504 on timeout.
func Timeout(timeout time.Duration) fwapp.Middleware {
	if timeout <= 0 {
		panic("timeout must be > 0")
	}

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
			defer cancel()

			c.Request = c.Request.WithContext(ctx)
			err := next(c)

			if ctx.Err() == context.DeadlineExceeded && err == nil {
				return fwctx.NewHTTPError(http.StatusGatewayTimeout, "request timed out", ctx.Err())
			}

			return err
		}
	}
}

// CORS adds basic CORS response headers and handles OPTIONS preflight.
func CORS(config CORSConfig) fwapp.Middleware {
	normalized := normalizeCORSConfig(config)

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			c.SetHeader("Vary", "Origin")
			c.SetHeader("Access-Control-Allow-Origin", normalized.AllowOrigin)
			c.SetHeader("Access-Control-Allow-Methods", normalized.AllowMethods)
			c.SetHeader("Access-Control-Allow-Headers", normalized.AllowHeaders)

			if normalized.AllowCredentials {
				c.SetHeader("Access-Control-Allow-Credentials", "true")
			}
			if normalized.MaxAgeSeconds > 0 {
				c.SetHeader("Access-Control-Max-Age", strconv.Itoa(normalized.MaxAgeSeconds))
			}

			if c.Request.Method == http.MethodOptions {
				c.Status(http.StatusNoContent)
				return nil
			}

			return next(c)
		}
	}
}

// SecurityHeaders adds default security headers.
func SecurityHeaders(config SecurityHeadersConfig) fwapp.Middleware {
	normalized := normalizeSecurityHeadersConfig(config)

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			setHeaderIfMissing(c, "Content-Security-Policy", normalized.ContentSecurityPolicy)
			setHeaderIfMissing(c, "Referrer-Policy", normalized.ReferrerPolicy)
			setHeaderIfMissing(c, "X-Frame-Options", normalized.XFrameOptions)
			setHeaderIfMissing(c, "X-Content-Type-Options", normalized.XContentTypeOptions)
			setHeaderIfMissing(c, "Permissions-Policy", normalized.PermissionsPolicy)
			setHeaderIfMissing(c, "Strict-Transport-Security", normalized.StrictTransportSecurity)
			return next(c)
		}
	}
}

// RateLimit applies a simple fixed-window in-memory limiter.
func RateLimit(config RateLimitConfig) fwapp.Middleware {
	normalized := normalizeRateLimitConfig(config)

	type bucket struct {
		count int
		reset time.Time
	}

	var (
		mu      sync.Mutex
		buckets = map[string]*bucket{}
	)

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			key := normalized.KeyFunc(c)
			if strings.TrimSpace(key) == "" {
				key = "global"
			}

			now := time.Now()
			mu.Lock()
			current := buckets[key]
			if current == nil || now.After(current.reset) {
				current = &bucket{
					count: 0,
					reset: now.Add(normalized.Window),
				}
				buckets[key] = current
			}

			if current.count >= normalized.Requests {
				retryAfter := int(time.Until(current.reset).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				mu.Unlock()

				c.SetHeader("Retry-After", strconv.Itoa(retryAfter))
				return fwctx.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded", nil)
			}

			current.count++
			if len(buckets) > 10_000 {
				for candidateKey, candidateBucket := range buckets {
					if now.After(candidateBucket.reset) {
						delete(buckets, candidateKey)
					}
				}
			}
			mu.Unlock()

			return next(c)
		}
	}
}

// CSRF verifies that unsafe methods have matching cookie and token values.
func CSRF(config CSRFConfig) fwapp.Middleware {
	normalized := normalizeCSRFConfig(config)

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				return next(c)
			}

			cookie, err := c.Cookie(normalized.CookieName)
			if err != nil {
				return fwctx.NewHTTPError(http.StatusForbidden, "missing CSRF cookie", err)
			}

			token := strings.TrimSpace(c.Header(normalized.HeaderName))
			if token == "" {
				token = strings.TrimSpace(c.FormValue(normalized.FormField))
			}
			if token == "" || token != cookie.Value {
				return fwctx.NewHTTPError(http.StatusForbidden, "invalid CSRF token", nil)
			}

			return next(c)
		}
	}
}

func normalizeCORSConfig(config CORSConfig) CORSConfig {
	if config.AllowOrigin == "" {
		config.AllowOrigin = "*"
	}
	if config.AllowMethods == "" {
		config.AllowMethods = "GET,POST,PUT,PATCH,DELETE,OPTIONS"
	}
	if config.AllowHeaders == "" {
		config.AllowHeaders = "Content-Type,Authorization,X-Request-ID"
	}
	if config.MaxAgeSeconds < 0 {
		config.MaxAgeSeconds = 0
	}
	return config
}

func normalizeSecurityHeadersConfig(config SecurityHeadersConfig) SecurityHeadersConfig {
	if strings.TrimSpace(config.ContentSecurityPolicy) == "" {
		config.ContentSecurityPolicy = "default-src 'self'"
	}
	if strings.TrimSpace(config.ReferrerPolicy) == "" {
		config.ReferrerPolicy = "strict-origin-when-cross-origin"
	}
	if strings.TrimSpace(config.XFrameOptions) == "" {
		config.XFrameOptions = "DENY"
	}
	if strings.TrimSpace(config.XContentTypeOptions) == "" {
		config.XContentTypeOptions = "nosniff"
	}
	if strings.TrimSpace(config.PermissionsPolicy) == "" {
		config.PermissionsPolicy = "geolocation=(), microphone=(), camera=()"
	}
	if strings.TrimSpace(config.StrictTransportSecurity) == "" {
		config.StrictTransportSecurity = "max-age=31536000; includeSubDomains"
	}
	return config
}

func normalizeRateLimitConfig(config RateLimitConfig) RateLimitConfig {
	if config.Requests <= 0 {
		config.Requests = 60
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(c *fwctx.Context) string {
			host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			if err != nil {
				return c.Request.RemoteAddr
			}
			return host
		}
	}
	return config
}

func normalizeCSRFConfig(config CSRFConfig) CSRFConfig {
	if strings.TrimSpace(config.CookieName) == "" {
		config.CookieName = "csrf_token"
	}
	if strings.TrimSpace(config.HeaderName) == "" {
		config.HeaderName = "X-CSRF-Token"
	}
	if strings.TrimSpace(config.FormField) == "" {
		config.FormField = "_csrf"
	}
	return config
}

func setHeaderIfMissing(c *fwctx.Context, key, value string) {
	if strings.TrimSpace(c.Writer.Header().Get(key)) == "" {
		c.SetHeader(key, value)
	}
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}

	counter := atomic.AddUint64(&requestIDCounter, 1)
	return strconv.FormatUint(counter, 10)
}
