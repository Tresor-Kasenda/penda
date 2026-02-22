package testing

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// Client is a lightweight test HTTP client for framework apps.
type Client struct {
	handler http.Handler
}

// Response wraps recorder + request for assertions.
type Response struct {
	Recorder *httptest.ResponseRecorder
	Request  *http.Request
}

// TestingT is the subset of testing.T used by assertion helpers.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// NewClient creates a test client for the provided handler.
func NewClient(handler http.Handler) *Client {
	return &Client{handler: handler}
}

// Do executes an HTTP request against the handler.
func (c *Client) Do(req *http.Request) *Response {
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	return &Response{
		Recorder: rr,
		Request:  req,
	}
}

// Get performs a GET request.
func (c *Client) Get(path string) *Response {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	return c.Do(req)
}

// PostJSON performs a POST with JSON payload.
func (c *Client) PostJSON(path string, payload any) *Response {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

// PostForm performs a POST with x-www-form-urlencoded payload.
func (c *Client) PostForm(path string, values url.Values) *Response {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.Do(req)
}

// StatusCode returns response status code.
func (r *Response) StatusCode() int {
	return r.Recorder.Code
}

// Header returns response header value.
func (r *Response) Header(key string) string {
	return r.Recorder.Header().Get(key)
}

// BodyString returns response body as string.
func (r *Response) BodyString() string {
	return r.Recorder.Body.String()
}

// DecodeJSON decodes response body JSON into dst.
func (r *Response) DecodeJSON(dst any) error {
	return json.NewDecoder(strings.NewReader(r.BodyString())).Decode(dst)
}

// AssertStatus fails test if status does not match.
func AssertStatus(t TestingT, response *Response, expected int) {
	t.Helper()
	if response.StatusCode() != expected {
		t.Fatalf("expected status %d, got %d", expected, response.StatusCode())
	}
}

// AssertHeaderContains fails test if header does not contain substring.
func AssertHeaderContains(t TestingT, response *Response, key, contains string) {
	t.Helper()
	value := response.Header(key)
	if !strings.Contains(value, contains) {
		t.Fatalf("expected header %q to contain %q, got %q", key, contains, value)
	}
}
