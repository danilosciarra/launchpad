// Command minimal-http shows the smallest possible Launchpad service: an
// HTTP server with one route and graceful shutdown, wired up in a handful
// of lines.
//
// Run it with:
//
//	go run ./examples/minimal-http
//
// then, in another shell:
//
//	curl localhost:8080/health
//	curl localhost:8080/orders
package main

import (
	"log"
	"net/http"

	"github.com/danilosciarra/launchpad"
	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/httpserver"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := &config.Config{
		Name:    "orders",
		Version: "1.0.0",
		HTTP:    config.Address{Host: "0.0.0.0", Port: 8080},
	}

	app, err := launchpad.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = app.RegisterRoutes("/orders", httpserver.Route{
		Method: http.MethodGet,
		Path:   "",
		Handler: func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"orders": []string{}})
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Run blocks until SIGINT/SIGTERM, then shuts down gracefully.
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
