package main

import (
	"log"
	"net/http"

	"penda/framework/app"
	fwctx "penda/framework/context"
	"penda/framework/middleware"
	"penda/framework/observability"
)

func main() {
	server := app.New()
	server.Use(middleware.Recovery(), middleware.RequestID(), middleware.Logger(nil))

	server.Get("/", func(c *fwctx.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "hello from penda"})
	})
	server.Get("/health", observability.HealthHandler())
	server.Get("/hello/:name", func(c *fwctx.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"hello": c.Param("name")})
	})

	log.Println("hello example listening on :8081")
	log.Fatal(server.Run(":8081"))
}
