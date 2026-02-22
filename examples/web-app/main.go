package main

import (
	"log"
	"net/http"

	"penda/framework/app"
	fwctx "penda/framework/context"
	"penda/framework/middleware"
)

func main() {
	server := app.New()
	server.Use(
		middleware.Recovery(),
		middleware.Logger(nil),
		middleware.SecurityHeaders(middleware.SecurityHeadersConfig{}),
	)

	if err := server.LoadTemplates("templates/*.tmpl"); err != nil {
		log.Fatal(err)
	}
	server.Static("/assets", "./public")

	server.Get("/", func(c *fwctx.Context) error {
		return c.Render("index.tmpl", map[string]any{
			"Title": "Penda Web App",
			"Items": []string{"Templates", "Static files", "Middleware"},
		})
	})

	server.Get("/ping", func(c *fwctx.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Println("web-app example listening on :8082")
	log.Fatal(server.Run(":8082"))
}
