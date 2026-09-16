package httpserver

import "github.com/gin-gonic/gin"

// Route describes a single HTTP endpoint to register with a Server.
type Route struct {
	// Method is the HTTP method, e.g. http.MethodGet.
	Method string
	// Path is the route path, relative to the group it is registered under.
	Path string
	// Handler is the Gin handler invoked for this route. Chain your own
	// authentication or validation middleware into it directly, e.g.
	// Handler: authMiddleware(myHandler).
	Handler gin.HandlerFunc
}
