// Package log declares the minimal logging interface Launchpad depends on.
// Applications are never required to import this package directly: a
// default, sensibly-configured implementation is wired in automatically. It
// only needs to be imported to supply a custom logger via
// launchpad.WithLogger.
package log

// Logger is the small structured-logging interface Launchpad's built-in
// components (HTTP request logging, gRPC interceptors, server lifecycle
// messages) log through. Any logger can be adapted to it; wrap your logger
// of choice (logrus, zap, slog, ...) in a handful of lines.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)

	// WithFields returns a Logger that annotates every subsequent message
	// with the given structured fields.
	WithFields(fields Fields) Logger
}

// Fields is a set of structured key/value pairs attached to a log entry.
type Fields map[string]any
