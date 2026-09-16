# Launchpad

[![Go Reference](https://pkg.go.dev/badge/github.com/danilosciarra/launchpad.svg)](https://pkg.go.dev/github.com/danilosciarra/launchpad)
[![License: MIT](https://img.shields.io/github/license/danilosciarra/launchpad)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danilosciarra/launchpad)](go.mod)
[![CI](https://github.com/danilosciarra/launchpad/actions/workflows/ci.yml/badge.svg)](https://github.com/danilosciarra/launchpad/actions/workflows/ci.yml)

Launchpad is a lightweight bootstrap SDK for production-ready Go
microservices. It wires together a [Gin](https://github.com/gin-gonic/gin)
HTTP server, an optional gRPC server, optional [Dapr](https://dapr.io)
sidecar integration, and graceful startup/shutdown, behind a small API you
can learn in five minutes.

| Feature | How you get it |
|---|---|
| HTTP server (Gin) + `GET /health` | always on |
| gRPC server | set `config.Config.GRPC` |
| Dapr sidecar client | set `config.Config.Dapr` |
| Panic recovery + request logging (HTTP & gRPC) | always on |
| Custom middleware / interceptors | `WithMiddleware`, `WithUnaryInterceptor` |
| Swagger UI | `WithSwagger` |
| Custom logger (logrus, zap, slog, ...) | `WithLogger` |
| Coordinated graceful shutdown | `Run` / `Shutdown` |
| JSON config with env expansion + validation | `config.Load` / `config.LoadFile` |

**Requirements:** Go 1.26 or later.

## Install

```sh
go get github.com/danilosciarra/launchpad
```

## Quickstart

```go
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

	if err := app.Run(); err != nil { // blocks until SIGINT/SIGTERM
		log.Fatal(err)
	}
}
```

```sh
go run .
curl localhost:8080/health    # -> ok
curl localhost:8080/orders    # -> {"orders":[]}
```

Runnable examples live in `examples/`:

- `examples/minimal-http` — the program above.
- `examples/http-grpc` — HTTP + gRPC together, custom middleware, and the
  standard gRPC health service.
- `examples/with-dapr` — optional Dapr sidecar integration.

## Configuration

`config.Config` is framework-agnostic: it has no dependency on Gin, gRPC, or
Dapr, so it can be reused anywhere you need typed, validated configuration.

| Field | JSON key | Required | Notes |
|---|---|---|---|
| `Name` | `name` | yes | application name |
| `Description` | `description` | no | used in the Swagger spec |
| `Version` | `version` | yes | used in the Swagger spec |
| `HTTP` | `http` | yes | `{host, port}` the HTTP server listens on |
| `GRPC` | `grpc` | no | nil disables the gRPC server |
| `Dapr` | `dapr` | no | nil disables Dapr |
| `Log` | `log` | no | `level`, `format` (`json`/`text`), `output` (`stdout`/`file`) |
| `ShutdownTimeout` | `shutdownTimeout` | no | defaults to `30s` |

Build the struct in code (as in the quickstart) or load it from JSON:

```go
var cfg config.Config
if err := config.Load(&cfg); err != nil { // ./config.json next to the binary
	log.Fatal(err)
}
// or from an explicit path:
// err := config.LoadFile("/etc/orders/config.json", &cfg)
```

`config.json` may reference environment variables with Go template syntax,
and fields tagged `validate:"..."` (see
[go-playground/validator](https://github.com/go-playground/validator)) are
checked automatically after loading, so a missing required field fails fast
with a descriptive error.

```json
{
  "name": "orders",
  "description": "Orders service",
  "version": "1.0.0",
  "http": { "host": "0.0.0.0", "port": {{.PORT}} },
  "grpc": { "host": "0.0.0.0", "port": 9090 },
  "log": { "level": "info", "format": "json", "output": "stdout" },
  "shutdownTimeout": "30s"
}
```

> Values produced by template expansion are strings; Launchpad coerces them
> into the target field type (`int`, `bool`, `time.Duration`, ...), so
> `"port": "{{.PORT}}"` works as well.

Embed `config.Config` in your own type to add application-specific fields
while keeping `Load`/`LoadFile` working unchanged:

```go
type AppConfig struct {
	config.Config
	FeatureFlags map[string]bool `json:"featureFlags"`
}
```

See `config.example.json` for a complete file.

## HTTP routes

Routes are registered per group; middleware registered for that group with
`WithMiddleware` applies to all of them. Supported methods are GET, POST,
PUT, DELETE and PATCH — anything else returns an error, as does a nil
handler.

```go
err := app.RegisterRoutes("/orders",
	httpserver.Route{Method: http.MethodGet, Path: "", Handler: listOrders},
	httpserver.Route{Method: http.MethodGet, Path: "/:id", Handler: getOrder},
	httpserver.Route{Method: http.MethodPost, Path: "", Handler: createOrder},
)
```

`GET /health` is mounted automatically and returns `200 ok` — enough for
container orchestrator liveness/readiness probes. Register your own handler
on the same path to override it.

For anything `RegisterRoutes` does not cover (static files, `NoRoute`,
websockets, ...) reach for the raw engine:

```go
app.Engine().NoRoute(func(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
})
```

## gRPC

Set `cfg.GRPC` to enable the server, then register services with the
registrars generated by `protoc`:

```go
cfg.GRPC = &config.Address{Host: "0.0.0.0", Port: 9090}
app, _ := launchpad.New(cfg)

err := app.RegisterGRPCService(func(s *grpc.Server) {
	orderspb.RegisterOrdersServer(s, &ordersHandler{})
})
```

`RegisterGRPCService` returns an error if `cfg.GRPC` was not set. Panic
recovery and request logging interceptors are installed automatically; your
own interceptors run after them, in registration order.

## Middleware and interceptors

Extend the built-in HTTP and gRPC pipelines with functional options passed
to `launchpad.New`:

```go
app, err := launchpad.New(cfg,
	launchpad.WithMiddleware("/orders", authMiddleware), // "" = root group
	launchpad.WithUnaryInterceptor(myAuthInterceptor),
)
```

Authentication itself is intentionally not part of Launchpad: bring your own
JWT/OIDC/session middleware and register it the same way you would with
plain Gin or gRPC. Per-route middleware can also be chained directly into a
`Route.Handler`.

## Logging

By default Launchpad logs JSON to stdout, honoring `config.Config.Log`
(`level`, `format`, `output`; `output: "file"` writes to a rotating
`./app.log`). To plug in your own logger, implement `log.Logger` and pass it
with `WithLogger` — `config.Config.Log` is then ignored.

```go
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	WithFields(fields log.Fields) log.Logger
}

app, err := launchpad.New(cfg, launchpad.WithLogger(myLogger))
```

`app.Logger()` returns the active logger, so application code can share it.

## Swagger

```go
app, err := launchpad.New(cfg, launchpad.WithSwagger(docs.SwaggerInfo))
```

`docs.SwaggerInfo` is the `*swag.Spec` generated by
[swaggo/swag](https://github.com/swaggo/swag) (`swag init`). Launchpad fills
in `Title`, `Version`, `Description`, `BasePath` and `Schemes` from your
`config.Config` and mounts the UI at `GET /swagger/*any`.

## Dapr

Set `config.Config.Dapr` to connect to a sidecar at startup:

```go
cfg.Dapr = &config.Address{Host: "127.0.0.1", Port: 50001}
app, err := launchpad.New(cfg) // fails if the sidecar is unreachable

client := app.Dapr() // nil if config.Config.Dapr was not set
err = client.Client().PublishEvent(ctx, "pubsub", "orders.created", payload)
```

Launchpad only handles connecting and disconnecting the sidecar client as
part of application startup/shutdown. Everything else — pub/sub
subscriptions, state, secrets, service invocation, scheduled jobs — is the
full [`dapr/go-sdk`](https://github.com/dapr/go-sdk) client, available via
`Client.Client()`.

## Lifecycle

`app.Run()` starts the gRPC server (if enabled) and the HTTP server, then
blocks until `SIGINT`/`SIGTERM`, at which point it calls `Shutdown` for you.
Shutdown stops the gRPC server, the HTTP server and the Dapr client, bounded
by `config.Config.ShutdownTimeout` (default 30s), joining any errors.

For programmatic control (integration tests, custom signal handling) start
the servers yourself instead of calling `Run`:

```go
app, _ := launchpad.New(cfg)
_ = app.HTTPServer().Start() // returns immediately
defer app.Shutdown(context.Background())
```

## API at a glance

| Symbol | Purpose |
|---|---|
| `launchpad.New(cfg, opts...)` | build the application |
| `App.RegisterRoutes(group, routes...)` | register HTTP endpoints |
| `App.RegisterGRPCService(register)` | register a gRPC service |
| `App.Run()` / `App.Shutdown(ctx)` | lifecycle |
| `App.Engine()` | underlying `*gin.Engine` |
| `App.HTTPServer()` / `App.GRPCServer()` | underlying servers (gRPC may be nil) |
| `App.Dapr()` / `App.Logger()` | Dapr client (may be nil) / active logger |
| `WithLogger`, `WithSwagger`, `WithMiddleware`, `WithUnaryInterceptor` | options |

## Package layout

| Package                              | Purpose                                              |
|---------------------------------------|-------------------------------------------------------|
| `github.com/danilosciarra/launchpad`  | `App`, functional options, lifecycle (`Run`/`Shutdown`) |
| `.../launchpad/config`                | Framework-agnostic configuration types and loading    |
| `.../launchpad/httpserver`            | Gin-based HTTP server                                 |
| `.../launchpad/grpcserver`            | gRPC server                                           |
| `.../launchpad/dapr`                  | Optional Dapr sidecar client bootstrap                |
| `.../launchpad/log`                   | Minimal `Logger` interface for plugging in your own logger |

`httpserver` and `grpcserver` are usable standalone if you only need one
half of Launchpad. Everything else (default logging, built-in
middleware/interceptors) lives under `internal/` and is not part of the
public API.

## Contributing

Issues and pull requests are welcome. Run `golangci-lint run` and
`go test ./...` before submitting.

## License

[MIT](LICENSE)
