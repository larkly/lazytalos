package talos

import (
	"context"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
)

// Client wraps the siderolabs machinery gRPC client with context metadata.
type Client struct {
	ContextName string
	Endpoints   []string
	C           *talosclient.Client
}

// Connect creates a new connected Talos client from the given talosconfig file and context name.
// If contextName is empty, the default context from the talosconfig is used.
func Connect(ctx context.Context, talosconfig string, contextName string) (*Client, error) {
	opts := []talosclient.OptionFunc{
		talosclient.WithConfigFromFile(talosconfig),
	}

	if contextName != "" {
		opts = append(opts, talosclient.WithContextName(contextName))
	}

	c, err := talosclient.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Determine effective context name and endpoints from talosconfig.
	cfg, err := config.Open(talosconfig)
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	effectiveContext := contextName
	if effectiveContext == "" {
		effectiveContext = cfg.Context
	}

	var endpoints []string
	if ctx, ok := cfg.Contexts[effectiveContext]; ok {
		endpoints = ctx.Endpoints
	}

	return &Client{
		ContextName: effectiveContext,
		Endpoints:   endpoints,
		C:           c,
	}, nil
}

// Close closes the underlying machinery client.
func (c *Client) Close() error {
	return c.C.Close()
}
