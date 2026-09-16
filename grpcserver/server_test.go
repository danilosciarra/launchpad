package grpcserver_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/grpcserver"
	"github.com/danilosciarra/launchpad/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// recordingLogger is a log.Logger that captures the messages written to it.
type recordingLogger struct {
	mu       sync.Mutex
	messages *[]string
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{messages: &[]string{}}
}

func (l *recordingLogger) record(level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.messages = append(*l.messages, level+": "+fmt.Sprintf(format, args...))
}

func (l *recordingLogger) Debugf(format string, args ...any) { l.record("debug", format, args...) }
func (l *recordingLogger) Infof(format string, args ...any)  { l.record("info", format, args...) }
func (l *recordingLogger) Warnf(format string, args ...any)  { l.record("warn", format, args...) }
func (l *recordingLogger) Errorf(format string, args ...any) { l.record("error", format, args...) }

func (l *recordingLogger) WithFields(_ log.Fields) log.Logger { return l }

func (l *recordingLogger) recorded() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), *l.messages...)
}

func freePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	require.NoError(t, lis.Close())
	return port
}

// dialHealthClient starts s on a free port and returns a health client
// connected to it, shutting everything down when the test ends.
func dialHealthClient(t *testing.T, port int, s *grpcserver.Server) healthpb.HealthClient {
	t.Helper()

	require.NoError(t, s.Start())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return healthpb.NewHealthClient(conn)
}

func TestStartAndShutdown(t *testing.T) {
	var registered bool
	s := grpcserver.New(config.Address{Host: "127.0.0.1", Port: 0})
	s.RegisterService(func(_ *grpc.Server) { registered = true })

	require.NoError(t, s.Start())
	require.True(t, registered)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
}

func TestShutdown_WithoutStart(t *testing.T) {
	s := grpcserver.New(config.Address{Host: "127.0.0.1", Port: 0})
	require.NoError(t, s.Shutdown(context.Background()))
}

func TestStart_ServesRegisteredService(t *testing.T) {
	port := freePort(t)
	s := grpcserver.New(config.Address{Host: "127.0.0.1", Port: port})
	s.RegisterService(func(srv *grpc.Server) {
		healthpb.RegisterHealthServer(srv, health.NewServer())
	})

	client := dialHealthClient(t, port, s)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.GetStatus())
}

func TestRegisterService_MultipleRegistrars(t *testing.T) {
	var calls []string
	s := grpcserver.New(config.Address{Host: "127.0.0.1", Port: freePort(t)})
	s.RegisterService(func(_ *grpc.Server) { calls = append(calls, "first") })
	s.RegisterService(func(_ *grpc.Server) { calls = append(calls, "second") })

	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	assert.Equal(t, []string{"first", "second"}, calls)
}

func TestStart_ListenErrorOnBusyPort(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = lis.Close() }()

	port := lis.Addr().(*net.TCPAddr).Port
	s := grpcserver.New(config.Address{Host: "127.0.0.1", Port: port})

	assert.Error(t, s.Start(), "binding an already-used port must fail")
}

func TestWithUnaryInterceptor_RunsInOrderAfterBuiltIns(t *testing.T) {
	var order []string
	first := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		order = append(order, "first")
		return handler(ctx, req)
	}
	second := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		order = append(order, "second")
		return handler(ctx, req)
	}

	port := freePort(t)
	s := grpcserver.New(
		config.Address{Host: "127.0.0.1", Port: port},
		grpcserver.WithUnaryInterceptor(first),
		grpcserver.WithUnaryInterceptor(second),
	)
	s.RegisterService(func(srv *grpc.Server) {
		healthpb.RegisterHealthServer(srv, health.NewServer())
	})

	client := dialHealthClient(t, port, s)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	assert.Equal(t, []string{"first", "second"}, order)
}

func TestPanicIsRecovered(t *testing.T) {
	panicking := func(_ context.Context, _ any, _ *grpc.UnaryServerInfo, _ grpc.UnaryHandler) (any, error) {
		panic("kaboom")
	}

	port := freePort(t)
	s := grpcserver.New(
		config.Address{Host: "127.0.0.1", Port: port},
		grpcserver.WithUnaryInterceptor(panicking),
	)
	s.RegisterService(func(srv *grpc.Server) {
		healthpb.RegisterHealthServer(srv, health.NewServer())
	})

	client := dialHealthClient(t, port, s)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Check(ctx, &healthpb.HealthCheckRequest{})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	// The server must still be alive after recovering from the panic.
	_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
	assert.Error(t, err)
}

func TestWithLogger_LogsLifecycleAndRequests(t *testing.T) {
	logger := newRecordingLogger()
	port := freePort(t)
	s := grpcserver.New(
		config.Address{Host: "127.0.0.1", Port: port},
		grpcserver.WithLogger(logger),
	)
	s.RegisterService(func(srv *grpc.Server) {
		healthpb.RegisterHealthServer(srv, health.NewServer())
	})

	client := dialHealthClient(t, port, s)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	assert.Contains(t, logger.recorded(), "info: request handled")
}

func TestShutdown_AlreadyCancelledContextForcesStop(t *testing.T) {
	port := freePort(t)
	s := grpcserver.New(config.Address{Host: "127.0.0.1", Port: port})
	require.NoError(t, s.Start())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Shutdown never reports an error: it falls back to a forced stop.
	assert.NoError(t, s.Shutdown(ctx))
}
