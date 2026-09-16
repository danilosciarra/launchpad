package grpcinterceptor_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/danilosciarra/launchpad/internal/grpcinterceptor"
	"github.com/danilosciarra/launchpad/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type entry struct {
	level   string
	message string
	fields  log.Fields
}

// recordingLogger captures entries, merging the fields accumulated through
// successive WithFields calls.
type recordingLogger struct {
	mu      *sync.Mutex
	entries *[]entry
	fields  log.Fields
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{mu: &sync.Mutex{}, entries: &[]entry{}, fields: log.Fields{}}
}

func (l *recordingLogger) record(level, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, entry{level: level, message: message, fields: l.fields})
}

func (l *recordingLogger) Debugf(format string, _ ...any) { l.record("debug", format) }
func (l *recordingLogger) Infof(format string, _ ...any)  { l.record("info", format) }
func (l *recordingLogger) Warnf(format string, _ ...any)  { l.record("warn", format) }
func (l *recordingLogger) Errorf(format string, _ ...any) { l.record("error", format) }

func (l *recordingLogger) WithFields(fields log.Fields) log.Logger {
	merged := log.Fields{}
	for k, v := range l.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &recordingLogger{mu: l.mu, entries: l.entries, fields: merged}
}

func (l *recordingLogger) last() entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return (*l.entries)[len(*l.entries)-1]
}

var info = &grpc.UnaryServerInfo{FullMethod: "/orders.Orders/Get"}

func TestRecovery_TurnsPanicIntoInternalError(t *testing.T) {
	logger := newRecordingLogger()
	interceptor := grpcinterceptor.Recovery(logger)

	var resp any
	var err error
	require.NotPanics(t, func() {
		resp, err = interceptor(context.Background(), "req", info, func(context.Context, any) (any, error) {
			panic("kaboom")
		})
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	got := logger.last()
	assert.Equal(t, "error", got.level)
	assert.Equal(t, "panic recovered", got.message)
	assert.Equal(t, "kaboom", got.fields["panic"])
	assert.Contains(t, got.fields, "stack")
}

func TestRecovery_PassesThroughWhenNoPanic(t *testing.T) {
	interceptor := grpcinterceptor.Recovery(newRecordingLogger())

	resp, err := interceptor(context.Background(), "req", info, func(context.Context, any) (any, error) {
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestLogging_Levels(t *testing.T) {
	tests := []struct {
		name        string
		handlerErr  error
		wantLevel   string
		wantMessage string
		wantCode    string
	}{
		{
			name:        "success",
			wantLevel:   "info",
			wantMessage: "request handled",
			wantCode:    codes.OK.String(),
		},
		{
			name:        "client error",
			handlerErr:  status.Error(codes.InvalidArgument, "bad id"),
			wantLevel:   "warn",
			wantMessage: "gRPC client error",
			wantCode:    codes.InvalidArgument.String(),
		},
		{
			name:        "server error",
			handlerErr:  status.Error(codes.Internal, "boom"),
			wantLevel:   "error",
			wantMessage: "gRPC server error",
			wantCode:    codes.Internal.String(),
		},
		{
			name:        "unavailable is a server error",
			handlerErr:  status.Error(codes.Unavailable, "no backend"),
			wantLevel:   "error",
			wantMessage: "gRPC server error",
			wantCode:    codes.Unavailable.String(),
		},
		{
			name:        "non-status error is treated as unknown",
			handlerErr:  errors.New("plain error"),
			wantLevel:   "error",
			wantMessage: "gRPC server error",
			wantCode:    codes.Unknown.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newRecordingLogger()
			interceptor := grpcinterceptor.Logging(logger)

			_, err := interceptor(context.Background(), "req", info, func(context.Context, any) (any, error) {
				return "resp", tt.handlerErr
			})
			assert.Equal(t, tt.handlerErr, err, "the handler error must be propagated unchanged")

			got := logger.last()
			assert.Equal(t, tt.wantLevel, got.level)
			assert.Equal(t, tt.wantMessage, got.message)
			assert.Equal(t, tt.wantCode, got.fields["grpcCode"])
			assert.Equal(t, info.FullMethod, got.fields["method"])
			assert.Contains(t, got.fields, "latency")
		})
	}
}

func TestLogging_PropagatesResponse(t *testing.T) {
	interceptor := grpcinterceptor.Logging(newRecordingLogger())

	resp, err := interceptor(context.Background(), "req", info, func(context.Context, any) (any, error) {
		return "resp", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "resp", resp)
}
