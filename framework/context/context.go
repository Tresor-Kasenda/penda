package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const defaultMultipartMemory int64 = 32 << 20 // 32 MiB

// TemplateRenderer renders a named template into the current response.
type TemplateRenderer func(w http.ResponseWriter, r *http.Request, code int, name string, data any) error

// HTTPError carries an HTTP status code with an error.
type HTTPError struct {
	Code    int
	Message string
	Err     error
}

// Error returns the public error message.
func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}

	if e.Code >= 100 && e.Code <= 599 {
		return http.StatusText(e.Code)
	}
	return http.StatusText(http.StatusInternalServerError)
}

// Unwrap returns the wrapped error.
func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StatusCode returns the associated HTTP status code.
func (e *HTTPError) StatusCode() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	if e.Code < 100 || e.Code > 599 {
		return http.StatusInternalServerError
	}
	return e.Code
}

// NewHTTPError creates an HTTPError.
func NewHTTPError(code int, message string, err error) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// BadRequest creates a 400 HTTPError.
func BadRequest(message string, err error) *HTTPError {
	return NewHTTPError(http.StatusBadRequest, message, err)
}

// Context stores request-scoped data and helpers.
type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
	Params  map[string]string
	Locals  map[string]any

	renderer TemplateRenderer
}

// New builds a context for a single request.
func New(w http.ResponseWriter, r *http.Request, params map[string]string) *Context {
	if params == nil {
		params = map[string]string{}
	}

	return &Context{
		Writer:  w,
		Request: r,
		Params:  params,
		Locals:  map[string]any{},
	}
}

// SetRenderer sets a template renderer for this request context.
func (c *Context) SetRenderer(renderer TemplateRenderer) {
	c.renderer = renderer
}

// Param returns a path param value.
func (c *Context) Param(key string) string {
	return c.Params[key]
}

// Query returns a query value from URL.
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// Header returns a request header value.
func (c *Context) Header(key string) string {
	return c.Request.Header.Get(key)
}

// FormValue returns form value (urlencoded or multipart).
func (c *Context) FormValue(key string) string {
	return c.Request.FormValue(key)
}

// Set stores a local value in the request context.
func (c *Context) Set(key string, value any) {
	c.Locals[key] = value
}

// Get loads a local value from the request context.
func (c *Context) Get(key string) (any, bool) {
	value, ok := c.Locals[key]
	return value, ok
}

// Status writes a status code.
func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

// StatusCode returns the currently written status code when available.
func (c *Context) StatusCode() int {
	type statusCoder interface {
		StatusCode() int
	}

	if writer, ok := c.Writer.(statusCoder); ok {
		return writer.StatusCode()
	}
	return 0
}

// SetHeader sets a response header.
func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

// SetCookie sets a response cookie.
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Writer, cookie)
}

// Cookie returns a request cookie.
func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// Text writes a plain text response.
func (c *Context) Text(code int, body string) error {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	_, err := c.Writer.Write([]byte(body))
	return err
}

// HTML writes an HTML response.
func (c *Context) HTML(code int, body string) error {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(code)
	_, err := c.Writer.Write([]byte(body))
	return err
}

// Render renders a named template with status 200.
func (c *Context) Render(name string, data any) error {
	return c.RenderStatus(http.StatusOK, name, data)
}

// RenderStatus renders a named template with the provided status code.
func (c *Context) RenderStatus(code int, name string, data any) error {
	if c.renderer == nil {
		return NewHTTPError(http.StatusInternalServerError, "template renderer is not configured", nil)
	}
	return c.renderer(c.Writer, c.Request, code, name, data)
}

// JSON writes a JSON response.
func (c *Context) JSON(code int, payload any) error {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	return json.NewEncoder(c.Writer).Encode(payload)
}

// Redirect writes an HTTP redirect.
func (c *Context) Redirect(code int, location string) {
	http.Redirect(c.Writer, c.Request, location, code)
}

// File serves a file to the client.
func (c *Context) File(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewHTTPError(http.StatusNotFound, "file not found", err)
		}
		return NewHTTPError(http.StatusInternalServerError, "failed to access file", err)
	}

	http.ServeFile(c.Writer, c.Request, path)
	return nil
}

// Download serves a file as an attachment.
func (c *Context) Download(path, filename string) error {
	if filename == "" {
		return BadRequest("filename is required", nil)
	}

	c.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.File(path)
}

// BindJSON decodes JSON body into dst and applies basic required validation.
func (c *Context) BindJSON(dst any) error {
	if err := ensureBindablePointer(dst); err != nil {
		return BadRequest(err.Error(), err)
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return BadRequest("empty JSON body", err)
		}
		return BadRequest("invalid JSON body", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return BadRequest("invalid JSON body", errors.New("multiple JSON values"))
		}
		return BadRequest("invalid JSON body", err)
	}

	if err := validateRequiredFields(dst); err != nil {
		return BadRequest(err.Error(), err)
	}

	return nil
}

// BindForm parses form values into a struct and applies basic required validation.
func (c *Context) BindForm(dst any) error {
	if err := ensureStructPointer(dst); err != nil {
		return BadRequest(err.Error(), err)
	}

	if err := c.Request.ParseForm(); err != nil {
		return BadRequest("invalid form body", err)
	}

	values := c.Request.PostForm
	if len(values) == 0 {
		values = c.Request.Form
	}

	if err := bindValues(dst, values); err != nil {
		return BadRequest(err.Error(), err)
	}
	if err := validateRequiredFields(dst); err != nil {
		return BadRequest(err.Error(), err)
	}

	return nil
}

// MultipartForm parses and returns multipart form data.
func (c *Context) MultipartForm(maxMemory int64) (*multipart.Form, error) {
	if maxMemory <= 0 {
		maxMemory = defaultMultipartMemory
	}

	if err := c.Request.ParseMultipartForm(maxMemory); err != nil {
		return nil, BadRequest("invalid multipart form data", err)
	}
	return c.Request.MultipartForm, nil
}

// FormFile returns an uploaded file by field name.
func (c *Context) FormFile(field string) (multipart.File, *multipart.FileHeader, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, nil, BadRequest("invalid multipart file", err)
	}
	return file, header, nil
}

func ensureBindablePointer(dst any) error {
	if dst == nil {
		return errors.New("destination cannot be nil")
	}
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("destination must be a non-nil pointer")
	}
	return nil
}

func ensureStructPointer(dst any) error {
	if err := ensureBindablePointer(dst); err != nil {
		return err
	}

	rv := reflect.ValueOf(dst)
	if rv.Elem().Kind() != reflect.Struct {
		return errors.New("destination must be a pointer to struct")
	}
	return nil
}

func bindValues(dst any, values url.Values) error {
	structValue := reflect.ValueOf(dst).Elem()
	structType := structValue.Type()

	for i := 0; i < structType.NumField(); i++ {
		fieldType := structType.Field(i)
		if !fieldType.IsExported() {
			continue
		}

		key, skip := bindingKey(fieldType)
		if skip {
			continue
		}

		fieldValues, exists := values[key]
		if !exists || len(fieldValues) == 0 {
			continue
		}

		fieldValue := structValue.Field(i)
		if !fieldValue.CanSet() {
			continue
		}

		if err := setFieldValue(fieldValue, fieldValues); err != nil {
			return fmt.Errorf("field %q: %w", fieldType.Name, err)
		}
	}

	return nil
}

func bindingKey(field reflect.StructField) (string, bool) {
	if tag, specified, skip := parseTagName(field.Tag.Get("form")); skip {
		return "", true
	} else if specified {
		return tag, false
	}
	if tag, specified, skip := parseTagName(field.Tag.Get("json")); skip {
		return "", true
	} else if specified {
		return tag, false
	}

	return strings.ToLower(field.Name), false
}

func parseTagName(tag string) (string, bool, bool) {
	if tag == "" {
		return "", false, false
	}

	name := strings.Split(tag, ",")[0]
	if name == "-" {
		return "", true, true
	}
	if name == "" {
		return "", false, false
	}
	return name, true, false
}

func setFieldValue(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setFieldValue(field.Elem(), values)
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(values[0])
		return nil
	case reflect.Bool:
		v, err := strconv.ParseBool(values[0])
		if err != nil {
			return err
		}
		field.SetBool(v)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(values[0], 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(v)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(values[0], 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(v)
		return nil
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(values[0], field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(v)
		return nil
	case reflect.Slice:
		elemType := field.Type().Elem()
		slice := reflect.MakeSlice(field.Type(), len(values), len(values))
		for i := range values {
			if err := setFieldValue(slice.Index(i), []string{values[i]}); err != nil {
				return err
			}
			if slice.Index(i).Type() != elemType {
				return errors.New("unsupported slice element type")
			}
		}
		field.Set(slice)
		return nil
	default:
		return errors.New("unsupported field type")
	}
}

func validateRequiredFields(dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return errors.New("destination cannot be nil")
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		fieldType := rt.Field(i)
		if !fieldType.IsExported() {
			continue
		}

		if !hasRequiredTag(fieldType.Tag.Get("validate")) {
			continue
		}

		if rv.Field(i).IsZero() {
			name := fieldType.Name
			if formName, ok, _ := parseTagName(fieldType.Tag.Get("form")); ok && formName != "" {
				name = formName
			} else if jsonName, ok, _ := parseTagName(fieldType.Tag.Get("json")); ok && jsonName != "" {
				name = jsonName
			}

			return fmt.Errorf("field %q is required", name)
		}
	}

	return nil
}

func hasRequiredTag(tag string) bool {
	if tag == "" {
		return false
	}

	for _, part := range strings.Split(tag, ",") {
		if strings.TrimSpace(part) == "required" {
			return true
		}
	}
	return false
}
