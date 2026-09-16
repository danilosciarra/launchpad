package dapr_test

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/dapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// fakeSidecar starts a bare gRPC server standing in for a Dapr sidecar and
// returns the address it listens on.
func fakeSidecar(t *testing.T) config.Address {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return config.Address{Host: "127.0.0.1", Port: lis.Addr().(*net.TCPAddr).Port}
}

// unusedPort returns a port nothing is listening on.
func unusedPort(t *testing.T) int {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	require.NoError(t, lis.Close())
	return port
}

// TestNew covers the whole Dapr bootstrap in a single, ordered test: the
// Dapr Go SDK caches its client process-wide, so the "no sidecar" case must
// run before any successful connection is established.
func TestNew(t *testing.T) {
	// New mutates the process environment; t.Setenv restores it afterwards.
	t.Setenv("DAPR_GRPC_ENDPOINT", "")

	var client *dapr.Client

	t.Run("fails when no sidecar is listening", func(t *testing.T) {
		_, err := dapr.New(config.Address{Host: "127.0.0.1", Port: unusedPort(t)})
		assert.Error(t, err)
	})

	addr := fakeSidecar(t)

	t.Run("connects to the sidecar", func(t *testing.T) {
		var err error
		client, err = dapr.New(addr)
		require.NoError(t, err)
		require.NotNil(t, client)

		assert.NotNil(t, client.Client(), "the underlying go-sdk client must be reachable")
	})

	t.Run("configures the sdk through DAPR_GRPC_ENDPOINT", func(t *testing.T) {
		assert.Equal(t, fmt.Sprintf("%s:%d", addr.Host, addr.Port), os.Getenv("DAPR_GRPC_ENDPOINT"))
	})

	t.Run("close is idempotent", func(t *testing.T) {
		require.NotNil(t, client)
		require.NoError(t, client.Close())
		assert.NoError(t, client.Close(), "closing twice must not fail")
	})
}

func TestClose_ZeroValueClient(t *testing.T) {
	var client dapr.Client
	assert.NoError(t, client.Close(), "closing a client that never connected must be a no-op")
}
