// Package config defines the framework-agnostic configuration shapes used to
// bootstrap a Launchpad application. It has no dependency on Gin, gRPC, or
// Dapr: it only describes data, so it can be reused by code that never
// imports the root launchpad package (for example, a CLI that only needs to
// read the log configuration).
package config

import "time"

// Config is the root configuration for a Launchpad application. Embed it in
// your own configuration struct, or use it directly with Load.
//
//	type AppConfig struct {
//	    config.Config
//	    FeatureFlags map[string]bool `json:"featureFlags"`
//	}
type Config struct {
	// Name is the application name. Required.
	Name string `json:"name" validate:"required"`
	// Description is a short, human readable description of the application.
	Description string `json:"description"`
	// Version is the application version. Required.
	Version string `json:"version" validate:"required"`

	// HTTP is the address the Gin HTTP server listens on. Required.
	HTTP Address `json:"http" validate:"required"`
	// GRPC is the address the gRPC server listens on. Leave nil to disable
	// the gRPC server entirely.
	GRPC *Address `json:"grpc,omitempty"`
	// Dapr is the address of the Dapr sidecar. Leave nil to disable Dapr
	// integration entirely.
	Dapr *Address `json:"dapr,omitempty"`

	// Log configures the default logger. Zero value uses sane defaults
	// (info level, JSON format, stdout).
	Log LogConfig `json:"log"`

	// ShutdownTimeout bounds how long graceful shutdown waits for in-flight
	// requests to finish before forcing a stop. Defaults to 30s when zero.
	ShutdownTimeout time.Duration `json:"shutdownTimeout"`
}

// Address identifies a host and port a server listens on or a client
// connects to.
type Address struct {
	Host string `json:"host" validate:"required"`
	Port int    `json:"port" validate:"required"`
}

// LogConfig configures the default logger used by a Launchpad application.
// It is ignored when a custom logger is supplied via launchpad.WithLogger.
type LogConfig struct {
	// Level is a logrus-compatible level name (e.g. "debug", "info",
	// "warn", "error"). Defaults to "info".
	Level string `json:"level"`
	// Format is either "json" (default) or "text".
	Format string `json:"format"`
	// Output is either "stdout" (default) or "file", which writes to a
	// rotating ./app.log.
	Output string `json:"output"`
}
