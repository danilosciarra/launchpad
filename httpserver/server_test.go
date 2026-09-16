package httpserver_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/httpserver"
	"github.com/danilosciarra/launchpad/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggo/swag"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// recordingLogger is a log.Logger that captures the messages written to it.
type recordingLogger struct {
	mu       sync.Mutex
	messages *[]string
	fields   log.Fields
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

func (l *recordingLogger) WithFields(fields log.Fields) log.Logger {
	return &recordingLogger{messages: l.messages, fields: fields}
}

func (l *recordingLogger) recorded() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), *l.messages...)
}

// freePort reserves and releases a port, returning a number that is very
// likely to still be free when the server binds it.
func freePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	require.NoError(t, lis.Close())
	return port
}

func TestNew_HealthCheck(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRegisterRoutes(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	err := s.RegisterRoutes("/orders", httpserver.Route{
		Method: http.MethodGet,
		Path:   "",
		Handler: func(c *gin.Context) {
			c.String(http.StatusOK, "orders")
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "orders", rec.Body.String())
}

func TestRegisterRoutes_NilHandlerRejected(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	err := s.RegisterRoutes("/orders", httpserver.Route{Method: http.MethodGet, Path: ""})
	assert.Error(t, err)
}

func TestWithMiddleware_AppliesToGroup(t *testing.T) {
	var called bool
	mw := func(c *gin.Context) {
		called = true
		c.Next()
	}

	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0}, httpserver.WithMiddleware("/orders", mw))
	require.NoError(t, s.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.True(t, called)
}

func TestRegisterRoutes_AllSupportedMethods(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	routes := make([]httpserver.Route, 0, len(methods))
	for _, m := range methods {
		routes = append(routes, httpserver.Route{
			Method:  m,
			Path:    "/" + m,
			Handler: func(c *gin.Context) { c.String(http.StatusOK, c.Request.Method) },
		})
	}
	require.NoError(t, s.RegisterRoutes("/api", routes...))

	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			req := httptest.NewRequest(m, "/api/"+m, nil)
			rec := httptest.NewRecorder()
			s.Engine().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, m, rec.Body.String())
		})
	}
}

func TestRegisterRoutes_UnsupportedMethodRejected(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	err := s.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodHead,
		Path:    "",
		Handler: func(c *gin.Context) {},
	})
	assert.Error(t, err)
}

func TestRegisterRoutes_NilHandlerRejectsWholeBatch(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	err := s.RegisterRoutes("/orders",
		httpserver.Route{
			Method:  http.MethodGet,
			Path:    "/ok",
			Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
		},
		httpserver.Route{Method: http.MethodGet, Path: "/bad"},
	)
	require.Error(t, err)

	// The valid route in the same batch must not have been registered.
	req := httptest.NewRequest(http.MethodGet, "/orders/ok", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWithMiddleware_RootGroup(t *testing.T) {
	var called bool
	s := httpserver.New(
		config.Address{Host: "127.0.0.1", Port: 0},
		httpserver.WithMiddleware("", func(c *gin.Context) { called = true; c.Next() }),
	)
	require.NoError(t, s.RegisterRoutes("", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "/ping",
		Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
	}))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

func TestWithMiddleware_DoesNotLeakToOtherGroups(t *testing.T) {
	var called bool
	s := httpserver.New(
		config.Address{Host: "127.0.0.1", Port: 0},
		httpserver.WithMiddleware("/orders", func(c *gin.Context) { called = true; c.Next() }),
	)
	require.NoError(t, s.RegisterRoutes("/users", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
	}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.False(t, called)
}

func TestWithMiddleware_AbortStopsHandler(t *testing.T) {
	var handlerCalled bool
	s := httpserver.New(
		config.Address{Host: "127.0.0.1", Port: 0},
		httpserver.WithMiddleware("/orders", func(c *gin.Context) {
			c.AbortWithStatus(http.StatusUnauthorized)
		}),
	)
	require.NoError(t, s.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { handlerCalled = true },
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, handlerCalled)
}

func TestPanicIsRecovered(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})
	require.NoError(t, s.RegisterRoutes("/boom", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { panic("kaboom") },
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() { s.Engine().ServeHTTP(rec, req) })
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWithSwagger_MountsUI(t *testing.T) {
	spec := &swag.Spec{
		InfoInstanceName: "TestSwagger",
		SwaggerTemplate:  `{"swagger":"2.0","info":{},"paths":{}}`,
	}
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0}, httpserver.WithSwagger(spec))

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestWithoutSwagger_RouteIsNotMounted(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	s.Engine().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStartAndShutdown_ServesRequests(t *testing.T) {
	port := freePort(t)
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: port})
	require.NoError(t, s.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.String(http.StatusOK, "orders") },
	}))

	require.NoError(t, s.Start())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})

	url := fmt.Sprintf("http://127.0.0.1:%d/orders", port)
	var resp *http.Response
	var err error
	for range 50 { // the listener starts asynchronously
		resp, err = http.Get(url) //nolint:gosec // test-local URL
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))

	_, err = http.Get(url) //nolint:gosec // test-local URL
	assert.Error(t, err, "server should refuse connections after shutdown")
}

func TestShutdown_WithoutStart(t *testing.T) {
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0})
	assert.NoError(t, s.Shutdown(context.Background()))
}

func TestWithLogger_LogsRequests(t *testing.T) {
	logger := newRecordingLogger()
	s := httpserver.New(config.Address{Host: "127.0.0.1", Port: 0}, httpserver.WithLogger(logger))
	require.NoError(t, s.RegisterRoutes("/orders", httpserver.Route{
		Method:  http.MethodGet,
		Path:    "",
		Handler: func(c *gin.Context) { c.Status(http.StatusOK) },
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	s.Engine().ServeHTTP(httptest.NewRecorder(), req)

	assert.Contains(t, logger.recorded(), "info: registered route: [GET] /orders")
	assert.Contains(t, logger.recorded(), "info: request handled")
}
