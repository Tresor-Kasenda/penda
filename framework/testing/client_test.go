package testing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

func TestClientGetAndPostJSON(t *testing.T) {
	server := fwapp.New()
	server.Get("/health", func(c *fwctx.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	server.Post("/echo", func(c *fwctx.Context) error {
		var payload map[string]string
		if err := c.BindJSON(&payload); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, payload)
	})

	client := NewClient(server)

	health := client.Get("/health")
	AssertStatus(t, health, http.StatusOK)

	echo := client.PostJSON("/echo", map[string]string{"name": "scott"})
	AssertStatus(t, echo, http.StatusOK)
	AssertHeaderContains(t, echo, "Content-Type", "application/json")
}

func TestAssertJSONEqualAndCookies(t *testing.T) {
	server := fwapp.New()

	server.Get("/json", func(c *fwctx.Context) error {
		c.SetCookie(&http.Cookie{
			Name:  "session",
			Value: "abc123",
			Path:  "/",
		})
		return c.JSON(http.StatusOK, map[string]any{
			"ok":   true,
			"user": "scott",
		})
	})

	server.Get("/echo-cookie", func(c *fwctx.Context) error {
		cookie, err := c.Cookie("session")
		if err != nil {
			return err
		}
		return c.Text(http.StatusOK, cookie.Value)
	})

	client := NewClient(server)

	resp := client.Get("/json")
	AssertStatus(t, resp, http.StatusOK)
	AssertJSONEqual(t, resp, map[string]any{
		"ok":   true,
		"user": "scott",
	})
	AssertCookieValue(t, resp, "session", "abc123")

	req := httptest.NewRequest(http.MethodGet, "/echo-cookie", nil)
	echo := client.DoWithCookies(req, &http.Cookie{Name: "session", Value: "abc123"})
	AssertStatus(t, echo, http.StatusOK)
	AssertBodyContains(t, echo, "abc123")
}

func TestPostMultipart(t *testing.T) {
	server := fwapp.New()
	server.Post("/upload", func(c *fwctx.Context) error {
		file, header, err := c.FormFile("avatar")
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{
			"filename": header.Filename,
			"size":     len(data),
			"name":     c.FormValue("name"),
		})
	})

	client := NewClient(server)
	resp := client.PostMultipart(
		"/upload",
		map[string]string{"name": "scott"},
		MultipartFile{
			FieldName: "avatar",
			FileName:  "avatar.txt",
			Content:   []byte("abc"),
		},
	)

	AssertStatus(t, resp, http.StatusOK)
	AssertJSONEqual(t, resp, map[string]any{
		"filename": "avatar.txt",
		"size":     3,
		"name":     "scott",
	})
}
