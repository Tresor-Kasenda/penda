package testing

import (
	"net/http"
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
