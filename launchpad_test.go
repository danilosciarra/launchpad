package launchpad_test

import (
	"testing"

	"github.com/danilosciarra/launchpad"
	"github.com/danilosciarra/launchpad/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func minimalConfig() *config.Config {
	return &config.Config{
		Name:    "orders",
		Version: "1.0.0",
		HTTP:    config.Address{Host: "127.0.0.1", Port: 0},
	}
}

func TestNew_NilConfig(t *testing.T) {
	_, err := launchpad.New(nil)
	assert.Error(t, err)
}

func TestNew_HTTPOnly(t *testing.T) {
	app, err := launchpad.New(minimalConfig())
	require.NoError(t, err)

	assert.NotNil(t, app.Engine())
	assert.Nil(t, app.GRPCServer())
	assert.Nil(t, app.Dapr())
}

func TestRegisterGRPCService_WithoutGRPCConfigured(t *testing.T) {
	app, err := launchpad.New(minimalConfig())
	require.NoError(t, err)

	err = app.RegisterGRPCService(func(_ *grpc.Server) {})
	assert.Error(t, err)
}

func TestNew_WithGRPC(t *testing.T) {
	cfg := minimalConfig()
	cfg.GRPC = &config.Address{Host: "127.0.0.1", Port: 0}

	app, err := launchpad.New(cfg)
	require.NoError(t, err)
	assert.NotNil(t, app.GRPCServer())

	require.NoError(t, app.RegisterGRPCService(func(_ *grpc.Server) {}))
}
