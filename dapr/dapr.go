// Package dapr provides optional bootstrap support for connecting to a Dapr
// sidecar. Launchpad only handles connecting and disconnecting the client as
// part of application startup/shutdown; for everything else (pub/sub,
// state, secrets, service invocation, jobs, ...) use the full
// github.com/dapr/go-sdk/client API via Client.Client.
//
// This package is a no-op unless your configuration sets a Dapr address, so
// it never requires a running sidecar for applications that don't use it.
package dapr

import (
	"fmt"
	"os"

	"github.com/danilosciarra/launchpad/config"

	daprclient "github.com/dapr/go-sdk/client"
)

// Client wraps a Dapr sidecar connection.
type Client struct {
	client daprclient.Client
}

// New connects to the Dapr sidecar at addr. It sets the DAPR_GRPC_ENDPOINT
// environment variable expected by the Dapr Go SDK before dialing, and
// returns an error if the sidecar cannot be reached.
//
// The Dapr Go SDK caches its client process-wide: once a connection has been
// established, later calls reuse it and ignore addr. This only matters to
// applications that build more than one Launchpad App in the same process.
func New(addr config.Address) (*Client, error) {
	endpoint := fmt.Sprintf("%s:%d", addr.Host, addr.Port)
	if err := os.Setenv("DAPR_GRPC_ENDPOINT", endpoint); err != nil {
		return nil, fmt.Errorf("dapr: set DAPR_GRPC_ENDPOINT: %w", err)
	}

	raw, err := daprclient.NewClient()
	if err != nil {
		return nil, fmt.Errorf("dapr: connect to sidecar at %s: %w", endpoint, err)
	}

	return &Client{client: raw}, nil
}

// Client returns the underlying github.com/dapr/go-sdk/client.Client, giving
// full access to pub/sub, state, secrets, service invocation and jobs APIs.
func (c *Client) Client() daprclient.Client {
	return c.client
}

// Close releases the sidecar connection.
func (c *Client) Close() error {
	if c.client != nil {
		c.client.Close()
	}
	return nil
}
