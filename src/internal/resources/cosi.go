// Package resources provides generic helpers for listing COSI resources across
// all Talos nodes and collecting typed results with their originating hostname.
package resources

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/talos"
)

// NodeResource wraps a COSI resource with its originating node hostname.
type NodeResource[T any] struct {
	NodeHostname string
	Resource     T
}

// listPerNode iterates over all configured endpoints, queries COSI for the given
// resource metadata kind from each node individually, and calls fn for each
// item.  fn receives the node address (used as a stand-in for the hostname when
// the caller hasn't resolved it yet) and the raw resource.Resource.
//
// Errors from individual nodes are collected and returned only if every node
// fails; partial results from healthy nodes are always included.
func listPerNode(
	ctx context.Context,
	c *talos.Client,
	md resource.Metadata,
	fn func(nodeAddr string, item resource.Resource),
) error {
	if c == nil || c.C == nil {
		return nil
	}

	var lastErr error
	successCount := 0

	for _, endpoint := range c.Endpoints {
		nodeCtx := talosclient.WithNode(ctx, endpoint)

		list, err := c.C.COSI.List(nodeCtx, md)
		if err != nil {
			lastErr = err
			continue
		}

		successCount++

		for _, item := range list.Items {
			fn(endpoint, item)
		}
	}

	if successCount == 0 && lastErr != nil {
		return lastErr
	}

	return nil
}
