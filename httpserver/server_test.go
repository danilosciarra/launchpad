package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/httpserver"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
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
