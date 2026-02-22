package context

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalsSetAndGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?q=go", nil)
	rr := httptest.NewRecorder()
	ctx := New(rr, req, map[string]string{"id": "42"})

	ctx.Set("trace_id", "abc123")

	value, ok := ctx.Get("trace_id")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if value != "abc123" {
		t.Fatalf("expected value %q, got %v", "abc123", value)
	}

	if param := ctx.Param("id"); param != "42" {
		t.Fatalf("expected param %q, got %q", "42", param)
	}

	if query := ctx.Query("q"); query != "go" {
		t.Fatalf("expected query %q, got %q", "go", query)
	}
}

func TestBindJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"scott","age":33}`))
	rr := httptest.NewRecorder()
	ctx := New(rr, req, nil)

	var payload struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age"`
	}
	if err := ctx.BindJSON(&payload); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if payload.Name != "scott" || payload.Age != 33 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestBindJSONValidationError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"age":33}`))
	rr := httptest.NewRecorder()
	ctx := New(rr, req, nil)

	var payload struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age"`
	}

	err := ctx.BindJSON(&payload)
	if err == nil {
		t.Fatal("expected error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, httpErr.StatusCode())
	}
}

func TestBindForm(t *testing.T) {
	body := strings.NewReader("name=scott&age=33&tags=go&tags=web")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	ctx := New(rr, req, nil)

	var payload struct {
		Name string   `form:"name" validate:"required"`
		Age  int      `form:"age"`
		Tags []string `form:"tags"`
	}
	if err := ctx.BindForm(&payload); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if payload.Name != "scott" || payload.Age != 33 || len(payload.Tags) != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestMultipartFormAndFile(t *testing.T) {
	body := &strings.Builder{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("name", "scott"); err != nil {
		t.Fatalf("write field: %v", err)
	}

	part, err := writer.CreateFormFile("avatar", "avatar.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader("avatar-content")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ctx := New(rr, req, nil)

	form, err := ctx.MultipartForm(1024)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if form.Value["name"][0] != "scott" {
		t.Fatalf("unexpected form value: %v", form.Value["name"])
	}

	file, header, err := ctx.FormFile("avatar")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "avatar-content" {
		t.Fatalf("unexpected file content: %q", string(content))
	}
	if header.Filename != "avatar.txt" {
		t.Fatalf("unexpected filename: %q", header.Filename)
	}
}

func TestResponseHelpers(t *testing.T) {
	t.Run("HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		ctx := New(rr, req, nil)

		if err := ctx.HTML(http.StatusAccepted, "<h1>Hello</h1>"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if rr.Code != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, rr.Code)
		}
		if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("unexpected content type: %q", rr.Header().Get("Content-Type"))
		}
	})

	t.Run("Redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		ctx := New(rr, req, nil)

		ctx.Redirect(http.StatusFound, "/next")

		if rr.Code != http.StatusFound {
			t.Fatalf("expected status %d, got %d", http.StatusFound, rr.Code)
		}
		if rr.Header().Get("Location") != "/next" {
			t.Fatalf("expected location %q, got %q", "/next", rr.Header().Get("Location"))
		}
	})
}

func TestFileAndDownload(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	t.Run("File", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		ctx := New(rr, req, nil)

		if err := ctx.File(filePath); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if strings.TrimSpace(rr.Body.String()) != "hello world" {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})

	t.Run("Download", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		ctx := New(rr, req, nil)

		if err := ctx.Download(filePath, "greeting.txt"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		header := rr.Header().Get("Content-Disposition")
		if !strings.Contains(header, "attachment") || !strings.Contains(header, "greeting.txt") {
			t.Fatalf("unexpected content-disposition: %q", header)
		}
	})
}

func TestCookies(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc"})
	rr := httptest.NewRecorder()
	ctx := New(rr, req, nil)

	cookie, err := ctx.Cookie("session_id")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cookie.Value != "abc" {
		t.Fatalf("expected cookie value %q, got %q", "abc", cookie.Value)
	}

	ctx.SetCookie(&http.Cookie{Name: "theme", Value: "light"})
	if !strings.Contains(rr.Header().Get("Set-Cookie"), "theme=light") {
		t.Fatalf("unexpected Set-Cookie header: %q", rr.Header().Get("Set-Cookie"))
	}
}

func TestRenderHelpers(t *testing.T) {
	t.Run("RenderWithoutRenderer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		ctx := New(rr, req, nil)

		err := ctx.Render("index.tmpl", map[string]string{"Name": "Scott"})
		if err == nil {
			t.Fatal("expected error when renderer is not configured")
		}
	})

	t.Run("RenderWithRenderer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		ctx := New(rr, req, nil)
		ctx.SetRenderer(func(w http.ResponseWriter, r *http.Request, code int, name string, data any) error {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(name))
			return nil
		})

		if err := ctx.RenderStatus(http.StatusCreated, "created.tmpl", nil); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
		}
		if rr.Body.String() != "created.tmpl" {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})
}
