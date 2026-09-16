// Command with-dapr shows optional Dapr sidecar integration: set
// config.Config.Dapr and Launchpad connects to the sidecar for you,
// exposing the full github.com/dapr/go-sdk/client API via app.Dapr().Client().
//
// This example expects a Dapr sidecar to be running alongside it, e.g.:
//
//	dapr run --app-id orders --dapr-grpc-port 50001 --app-port 8080 -- go run ./examples/with-dapr
package main

import (
	"context"
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
		Dapr:    &config.Address{Host: "127.0.0.1", Port: 50001},
	}

	app, err := launchpad.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = app.RegisterRoutes("/orders", httpserver.Route{
		Method: http.MethodPost,
		Path:   "",
		Handler: func(c *gin.Context) {
			// app.Dapr() is nil whenever config.Config.Dapr is unset, so
			// production code should guard this the same way.
			if app.Dapr() == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dapr not configured"})
				return
			}

			err := app.Dapr().Client().PublishEvent(context.Background(), "pubsub", "orders.created", []byte(`{"id":"1"}`))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.Status(http.StatusAccepted)
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
