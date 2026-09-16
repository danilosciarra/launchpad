// Package httpserver provides a Gin-based HTTP server with route-group
// middleware, a built-in health check and optional Swagger UI mounting. It
// is used internally by the root launchpad package, but is safe to use
// standalone when you only need the HTTP half of Launchpad.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/internal/httpmiddleware"
	internallogging "github.com/danilosciarra/launchpad/internal/logging"
	"github.com/danilosciarra/launchpad/log"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"
)

// Server wraps a Gin engine with lifecycle management (graceful start and
// stop) and group-scoped middleware registration.
type Server struct {
	engine *gin.Engine
	addr   string
	logger log.Logger
	server *http.Server

	middleware map[string][]gin.HandlerFunc
	swagger    *swag.Spec
}

// Option configures a Server. Pass options to New.
type Option func(*Server)

// WithLogger overrides the default logger used for request logging and
// server lifecycle messages.
func WithLogger(logger log.Logger) Option {
	return func(s *Server) { s.logger = logger }
}

// WithMiddleware registers Gin middleware that runs for every route
// registered under the given group (an empty group is the root group "/").
// It only takes effect when passed as an Option to New, before any routes
// are registered.
func WithMiddleware(group string, middleware ...gin.HandlerFunc) Option {
	return func(s *Server) {
		s.middleware[group] = append(s.middleware[group], middleware...)
	}
}

// WithSwagger mounts a Swagger UI at GET /swagger/*any, serving the given
// spec. Set spec.Title, spec.Version, spec.Description and spec.BasePath
// before passing it in; typically this is done for you by
// launchpad.WithSwagger.
func WithSwagger(spec *swag.Spec) Option {
	return func(s *Server) { s.swagger = spec }
}

// New builds a Server listening on addr once Start is called. The returned
// server always exposes GET /health; register additional routes with
// RegisterRoutes.
func New(addr config.Address, opts ...Option) *Server {
	s := &Server{
		addr:       fmt.Sprintf("%s:%d", addr.Host, addr.Port),
		logger:     internallogging.New(config.LogConfig{}),
		middleware: make(map[string][]gin.HandlerFunc),
	}
	for _, opt := range opts {
		opt(s)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(httpmiddleware.RequestLogger(s.logger))

	if s.swagger != nil {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		s.logger.Infof("swagger configured on /swagger/*any")
	}

	engine.GET("/health", healthCheck)

	s.engine = engine
	return s
}

// Engine returns the underlying Gin engine, for advanced use cases not
// covered by RegisterRoutes (e.g. gin.Engine.NoRoute).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// RegisterRoutes registers handlers under a path group, applying any
// middleware registered for that group via WithMiddleware.
func (s *Server) RegisterRoutes(group string, routes ...Route) error {
	for _, r := range routes {
		if r.Handler == nil {
			return fmt.Errorf("httpserver: handler for route %q cannot be nil", r.Path)
		}
	}

	rg := s.engine.Group(group)
	rg.Use(s.middleware[group]...)

	for _, r := range routes {
		switch r.Method {
		case http.MethodGet:
			rg.GET(r.Path, r.Handler)
		case http.MethodPost:
			rg.POST(r.Path, r.Handler)
		case http.MethodPut:
			rg.PUT(r.Path, r.Handler)
		case http.MethodDelete:
			rg.DELETE(r.Path, r.Handler)
		case http.MethodPatch:
			rg.PATCH(r.Path, r.Handler)
		default:
			return fmt.Errorf("httpserver: unsupported method %q for route %q", r.Method, r.Path)
		}
		s.logger.Infof("registered route: [%s] %s%s", r.Method, group, r.Path)
	}
	return nil
}

// Start begins serving HTTP requests in the background and returns
// immediately. Call Shutdown to stop it gracefully.
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.engine,
	}

	go func() {
		s.logger.Infof("HTTP server listening on %s", s.addr)
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Errorf("HTTP server on %s failed: %v", s.addr, err)
		}
	}()

	return nil
}

// Shutdown gracefully stops the server, waiting for in-flight requests to
// finish until ctx is done.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.logger.Infof("HTTP server shutting down...")
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}
	s.logger.Infof("HTTP server stopped")
	return nil
}
