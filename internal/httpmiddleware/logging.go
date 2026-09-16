// Package httpmiddleware holds the built-in Gin middleware Launchpad wires
// into every HTTP server automatically. It is internal because these
// middlewares are an implementation detail of httpserver.Server, not part of
// the public extensibility surface (use launchpad.WithMiddleware or
// httpserver.Route to add your own).
package httpmiddleware

import (
	"time"

	"github.com/danilosciarra/launchpad/log"
	"github.com/gin-gonic/gin"
)

// RequestLogger returns a Gin middleware that logs one structured entry per
// request: method, path, status code and latency. Server errors (5xx) are
// logged at error level, client errors (4xx) at warn level, everything else
// at info level.
func RequestLogger(logger log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		entry := logger.WithFields(log.Fields{
			"status":  status,
			"method":  c.Request.Method,
			"path":    path,
			"query":   query,
			"ip":      c.ClientIP(),
			"latency": latency.Milliseconds(),
		})

		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		switch {
		case status >= 500:
			entry.WithFields(log.Fields{"error": errorMessage}).Errorf("HTTP server error")
		case status >= 400:
			entry.WithFields(log.Fields{"error": errorMessage}).Warnf("HTTP client error")
		default:
			entry.Infof("request handled")
		}
	}
}
