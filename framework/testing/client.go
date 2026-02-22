package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"reflect"
	"strings"
)

// Client is a lightweight test HTTP client for framework apps.
type Client struct {
	handler http.Handler
}

// MultipartFile describes a file uploaded via multipart form-data in tests.
type MultipartFile struct {
	FieldName   string
	FileName    string
	Content     []byte
	ContentType string
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

// DoWithCookies executes a request after attaching one or more cookies.
func (c *Client) DoWithCookies(req *http.Request, cookies ...*http.Cookie) *Response {
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		req.AddCookie(cookie)
	}
	return c.Do(req)
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

// PostMultipart performs a POST with multipart form fields and files.
func (c *Client) PostMultipart(path string, fields map[string]string, files ...MultipartFile) *Response {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			panic(fmt.Sprintf("write multipart field %q: %v", key, err))
		}
	}

	for _, file := range files {
		if strings.TrimSpace(file.FieldName) == "" {
			panic("multipart file field name cannot be empty")
		}
		if strings.TrimSpace(file.FileName) == "" {
			file.FileName = "upload.bin"
		}

		part, err := createMultipartPart(writer, file)
		if err != nil {
			panic(fmt.Sprintf("create multipart file part %q: %v", file.FieldName, err))
		}
		if _, err := part.Write(file.Content); err != nil {
			panic(fmt.Sprintf("write multipart file part %q: %v", file.FieldName, err))
		}
	}

	if err := writer.Close(); err != nil {
		panic(fmt.Sprintf("close multipart body: %v", err))
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return c.Do(req)
}

func createMultipartPart(writer *multipart.Writer, file MultipartFile) (io.Writer, error) {
	if strings.TrimSpace(file.ContentType) == "" {
		return writer.CreateFormFile(file.FieldName, file.FileName)
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, file.FieldName, file.FileName))
	header.Set("Content-Type", file.ContentType)
	return writer.CreatePart(header)
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

// BodyBytes returns the raw response body.
func (r *Response) BodyBytes() []byte {
	return r.Recorder.Body.Bytes()
}

// Result returns an http.Response view of the recorder.
func (r *Response) Result() *http.Response {
	return r.Recorder.Result()
}

// Cookies returns cookies set by the response.
func (r *Response) Cookies() []*http.Cookie {
	return r.Result().Cookies()
}

// Cookie returns a named response cookie if present.
func (r *Response) Cookie(name string) (*http.Cookie, bool) {
	for _, cookie := range r.Cookies() {
		if cookie.Name == name {
			return cookie, true
		}
	}
	return nil, false
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

// AssertBodyContains fails test if body does not contain substring.
func AssertBodyContains(t TestingT, response *Response, contains string) {
	t.Helper()
	body := response.BodyString()
	if !strings.Contains(body, contains) {
		t.Fatalf("expected body to contain %q, got %q", contains, body)
	}
}

// AssertJSONEqual compares response JSON body to expected payload semantically.
func AssertJSONEqual(t TestingT, response *Response, expected any) {
	t.Helper()

	var actualValue any
	if err := json.Unmarshal(response.BodyBytes(), &actualValue); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body=%q)", err, response.BodyString())
	}

	expectedBytes, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal expected JSON: %v", err)
	}

	var expectedValue any
	if err := json.Unmarshal(expectedBytes, &expectedValue); err != nil {
		t.Fatalf("unmarshal normalized expected JSON: %v", err)
	}

	if !reflect.DeepEqual(expectedValue, actualValue) {
		t.Fatalf("unexpected JSON body\nexpected: %s\ngot:      %s", string(expectedBytes), response.BodyString())
	}
}

// AssertCookieValue fails test if a response cookie is missing or has a wrong value.
func AssertCookieValue(t TestingT, response *Response, name, expectedValue string) {
	t.Helper()
	cookie, ok := response.Cookie(name)
	if !ok {
		t.Fatalf("expected cookie %q to be set", name)
	}
	if cookie.Value != expectedValue {
		t.Fatalf("expected cookie %q to have value %q, got %q", name, expectedValue, cookie.Value)
	}
}

// ReadAll reads a response body/part and fails the test on error.
func ReadAll(t TestingT, r io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	return data
}
