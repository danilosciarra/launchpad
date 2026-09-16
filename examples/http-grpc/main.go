// Command http-grpc shows an HTTP + gRPC service running side by side,
// custom middleware/interceptors, and the standard gRPC health service
// registered via RegisterGRPCService.
//
// Run it with:
//
//	go run ./examples/http-grpc
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/danilosciarra/launchpad"
	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/httpserver"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// requestID is a trivial example of a custom Gin middleware wired in via
// launchpad.WithMiddleware.
func requestID(c *gin.Context) {
	c.Header("X-Request-Id", time.Now().Format(time.RFC3339Nano))
	c.Next()
}

func main() {
	cfg := &config.Config{
		Name:    "orders",
		Version: "1.0.0",
		HTTP:    config.Address{Host: "0.0.0.0", Port: 8080},
		GRPC:    &config.Address{Host: "0.0.0.0", Port: 9090},
	}

	app, err := launchpad.New(cfg, launchpad.WithMiddleware("/orders", requestID))
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

	// Register the standard gRPC health service so orchestrators can probe
	// the gRPC port the same way they probe /health over HTTP.
	healthServer := health.NewServer()
	err = app.RegisterGRPCService(func(s *grpc.Server) {
		healthpb.RegisterHealthServer(s, healthServer)
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
