// Package grpcserver provides a gRPC server with built-in panic recovery
// and request logging interceptors, plus support for chaining in your own
// unary interceptors. It is used internally by the root launchpad package,
// but is safe to use standalone when you only need the gRPC half of
// Launchpad.
package grpcserver

import (
	"context"
	"fmt"
	"net"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/internal/grpcinterceptor"
	internallogging "github.com/danilosciarra/launchpad/internal/logging"
	"github.com/danilosciarra/launchpad/log"

	"google.golang.org/grpc"
)

// Server wraps a *grpc.Server with lifecycle management (graceful start and
// stop).
type Server struct {
	addr         string
	logger       log.Logger
	interceptors []grpc.UnaryServerInterceptor
	registrars   []func(*grpc.Server)

	server *grpc.Server
}

// Option configures a Server. Pass options to New.
type Option func(*Server)

// WithLogger overrides the default logger used for request logging and
// server lifecycle messages.
func WithLogger(logger log.Logger) Option {
	return func(s *Server) { s.logger = logger }
}

// WithUnaryInterceptor appends a unary interceptor to the chain, after the
// built-in recovery and logging interceptors. Interceptors run in the order
// they are added.
func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return func(s *Server) { s.interceptors = append(s.interceptors, interceptor) }
}

// New builds a Server listening on addr once Start is called.
func New(addr config.Address, opts ...Option) *Server {
	s := &Server{
		addr:   fmt.Sprintf("%s:%d", addr.Host, addr.Port),
		logger: internallogging.New(config.LogConfig{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RegisterService registers a gRPC service using the ServiceDesc generated
// by protoc, e.g.:
//
//	grpcServer.RegisterService(func(s *grpc.Server) {
//	    myproto.RegisterMyServiceServer(s, myHandler)
//	})
func (s *Server) RegisterService(register func(*grpc.Server)) {
	s.registrars = append(s.registrars, register)
}

// Start begins serving gRPC requests in the background and returns
// immediately, or an error if the listener could not be created. Call
// Shutdown to stop it gracefully.
func (s *Server) Start() error {
	chain := append([]grpc.UnaryServerInterceptor{
		grpcinterceptor.Recovery(s.logger),
		grpcinterceptor.Logging(s.logger),
	}, s.interceptors...)

	s.server = grpc.NewServer(grpc.ChainUnaryInterceptor(chain...))
	for _, register := range s.registrars {
		register(s.server)
	}

	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpcserver: listen on %s: %w", s.addr, err)
	}

	go func() {
		s.logger.Infof("gRPC server listening on %s", s.addr)
		if err := s.server.Serve(lis); err != nil {
			s.logger.Errorf("gRPC server exited: %v", err)
		}
	}()

	return nil
}

// Shutdown gracefully stops the server, waiting for in-flight RPCs to
// finish until ctx is done, after which it forces a stop.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.logger.Infof("gRPC server shutting down...")

	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		s.logger.Infof("gRPC server stopped")
	case <-ctx.Done():
		s.server.Stop()
		s.logger.Infof("gRPC server force-stopped after shutdown timeout")
	}
	return nil
}
