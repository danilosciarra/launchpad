package launchpad_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/danilosciarra/launchpad"
	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/httpserver"
	"github.com/danilosciarra/launchpad/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggo/swag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
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

func TestNew_ExposesHTTPServerAndLogger(t *testing.T) {
	app, err := launchpad.New(minimalConfig())
	require.NoError(t, err)

	assert.NotNil(t, app.HTTPServer())
	assert.NotNil(t, app.Logger())
	assert.Same(t, app.Engine(), app.HTTPServer().Engine())
}

func TestRegisterRoutes(t *testing.T) {
	app, err := launchpad.New(minimalConfig())
	require.NoError(t, err)

	require.NoError(t, app.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.String(http.StatusOK, "orders") },
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	rec := httptest.NewRecorder()
	app.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "orders", rec.Body.String())
}

func TestRegisterRoutes_InvalidRouteRejected(t *testing.T) {
	app, err := launchpad.New(minimalConfig())
	require.NoError(t, err)

	assert.Error(t, app.RegisterRoutes("/orders", httpserver.Route{Method: http.MethodGet, Path: ""}))
}

func TestHealthEndpointIsAlwaysRegistered(t *testing.T) {
	app, err := launchpad.New(minimalConfig())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	app.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestWithLogger_IsUsedByTheApp(t *testing.T) {
	logger := newRecordingLogger()
	app, err := launchpad.New(minimalConfig(), launchpad.WithLogger(logger))
	require.NoError(t, err)

	assert.Same(t, log.Logger(logger), app.Logger())

	require.NoError(t, app.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
	}))
	assert.Contains(t, logger.recorded(), "info: registered route: [GET] /orders")
}

func TestWithMiddleware_IsAppliedToGroup(t *testing.T) {
	var called bool
	app, err := launchpad.New(minimalConfig(),
		launchpad.WithMiddleware("/orders", func(c *gin.Context) { called = true; c.Next() }),
	)
	require.NoError(t, err)

	require.NoError(t, app.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
	}))

	app.Engine().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders", nil))
	assert.True(t, called)
}

func TestWithSwagger_PopulatesSpecFromConfig(t *testing.T) {
	spec := &swag.Spec{
		InfoInstanceName: "AppSwagger",
		SwaggerTemplate:  `{"swagger":"2.0","info":{},"paths":{}}`,
	}

	cfg := minimalConfig()
	cfg.Description = "Orders service"

	app, err := launchpad.New(cfg, launchpad.WithSwagger(spec))
	require.NoError(t, err)

	assert.Equal(t, cfg.Name+" API", spec.Title)
	assert.Equal(t, cfg.Version, spec.Version)
	assert.Equal(t, cfg.Description, spec.Description)
	assert.Equal(t, "/", spec.BasePath)
	assert.Equal(t, []string{"http", "https"}, spec.Schemes)

	rec := httptest.NewRecorder()
	app.Engine().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestWithUnaryInterceptor_IsInvoked(t *testing.T) {
	var called bool
	interceptor := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		called = true
		return handler(ctx, req)
	}

	port := freePort(t)
	cfg := minimalConfig()
	cfg.GRPC = &config.Address{Host: "127.0.0.1", Port: port}

	app, err := launchpad.New(cfg, launchpad.WithUnaryInterceptor(interceptor))
	require.NoError(t, err)
	require.NoError(t, app.RegisterGRPCService(func(s *grpc.Server) {
		healthpb.RegisterHealthServer(s, health.NewServer())
	}))

	require.NoError(t, app.GRPCServer().Start())
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	assert.True(t, called)
}

func TestShutdown_WithoutStartIsSafe(t *testing.T) {
	cfg := minimalConfig()
	cfg.GRPC = &config.Address{Host: "127.0.0.1", Port: freePort(t)}

	app, err := launchpad.New(cfg)
	require.NoError(t, err)

	assert.NoError(t, app.Shutdown(context.Background()))
}

func TestShutdown_StopsStartedServers(t *testing.T) {
	httpPort := freePort(t)
	grpcPort := freePort(t)

	cfg := minimalConfig()
	cfg.HTTP = config.Address{Host: "127.0.0.1", Port: httpPort}
	cfg.GRPC = &config.Address{Host: "127.0.0.1", Port: grpcPort}
	cfg.ShutdownTimeout = 5 * time.Second

	app, err := launchpad.New(cfg)
	require.NoError(t, err)
	require.NoError(t, app.GRPCServer().Start())
	require.NoError(t, app.HTTPServer().Start())

	url := fmt.Sprintf("http://127.0.0.1:%d/health", httpPort)
	var resp *http.Response
	for range 50 {
		resp, err = http.Get(url) //nolint:gosec // test-local URL
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, err)

	require.NoError(t, app.Shutdown(context.Background()))

	_, err = http.Get(url) //nolint:gosec // test-local URL
	assert.Error(t, err, "HTTP server should be stopped")

	_, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", grpcPort), time.Second)
	assert.Error(t, err, "gRPC server should be stopped")
}

func TestNew_UnreachableDaprSidecarFails(t *testing.T) {
	cfg := minimalConfig()
	cfg.Dapr = &config.Address{Host: "127.0.0.1", Port: freePort(t)}

	// No sidecar is listening, so bootstrapping must fail loudly instead of
	// returning an App with a broken Dapr client.
	_, err := launchpad.New(cfg)
	assert.Error(t, err)
}
