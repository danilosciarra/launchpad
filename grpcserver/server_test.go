package grpcserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/grpcserver"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

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
