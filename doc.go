// Package launchpad is a lightweight bootstrap SDK for production-ready Go
// microservices. It wires together a Gin HTTP server, an optional gRPC
// server, optional Dapr sidecar integration, and graceful startup/shutdown
// behind a small, functional-options-driven API.
//
// A minimal service looks like this:
//
//	cfg := &config.Config{
//	    Name:    "orders",
//	    Version: "1.0.0",
//	    HTTP:    config.Address{Host: "0.0.0.0", Port: 8080},
//	}
//
//	app, err := launchpad.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	_ = app.RegisterRoutes("/orders", httpserver.Route{
//	    Method:  http.MethodGet,
//	    Path:    "",
//	    Handler: listOrders,
//	})
//
//	if err := app.Run(); err != nil {
//	    log.Fatal(err)
//	}
//
// Set cfg.GRPC to enable the gRPC server and cfg.Dapr to connect to a Dapr
// sidecar; both are entirely optional and nil by default. See the examples/
// directory for complete, runnable programs.
package launchpad
