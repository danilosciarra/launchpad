// Package grpcinterceptor holds the built-in gRPC interceptors Launchpad
// wires into every gRPC server automatically (panic recovery and request
// logging). It is internal: add your own interceptors via
// launchpad.WithUnaryInterceptor or grpcserver.WithUnaryInterceptor instead
// of depending on this package directly.
package grpcinterceptor

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/danilosciarra/launchpad/log"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery returns a UnaryServerInterceptor that turns panics into a gRPC
// Internal error instead of crashing the process, logging the panic value
// and stack trace first.
func Recovery(logger log.Logger) grpc.UnaryServerInterceptor {
	handler := func(p any) error {
		logger.WithFields(log.Fields{
			"panic": p,
			"stack": string(debug.Stack()),
		}).Errorf("panic recovered")
		return status.Errorf(codes.Internal, "internal server error")
	}
	return recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(handler))
}

// Logging returns a UnaryServerInterceptor that logs one structured entry
// per RPC: method, gRPC status code and latency.
func Logging(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		latency := time.Since(start)
		grpcStatus, _ := status.FromError(err)
		code := grpcStatus.Code()

		entry := logger.WithFields(log.Fields{
			"grpcCode": code.String(),
			"method":   info.FullMethod,
			"latency":  latency.Milliseconds(),
		})

		switch {
		case isServerError(code):
			entry.WithFields(log.Fields{"error": grpcStatus.Message()}).Errorf("gRPC server error")
		case code != codes.OK:
			entry.WithFields(log.Fields{"error": grpcStatus.Message()}).Warnf("gRPC client error")
		default:
			entry.Infof("request handled")
		}

		return resp, err
	}
}

func isServerError(code codes.Code) bool {
	switch code {
	case codes.Internal, codes.DataLoss, codes.Unavailable, codes.Unknown, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
