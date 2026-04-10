// Package resources provides generic helpers for listing COSI resources across
// all Talos nodes and collecting typed results with their originating hostname.
package resources

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/talos"
)

// NodeResource wraps a COSI resource with its originating node hostname.
type NodeResource[T any] struct {
	NodeHostname string
	Resource     T
}

// listPerNode queries COSI for the given resource metadata from each target
// node individually, calling fn for each item. fn receives the node address
// and the raw resource.
//
// targets defaults to c.Endpoints when nil. Callers that need all cluster
// nodes (not just endpoints) should pass addresses from cluster.NodeTargets().
// Note: cluster.GetMembers uses listPerNode with nil targets (endpoints only)
// to avoid circular dependency, since NodeTargets calls GetMembers.
func listPerNode(
	ctx context.Context,
	c *talos.Client,
	md resource.Metadata,
	fn func(nodeAddr string, item resource.Resource),
) error {
	return listPerNodeTargets(ctx, c, nil, md, fn)
}

func listPerNodeTargets(
	ctx context.Context,
	c *talos.Client,
	targets []string,
	md resource.Metadata,
	fn func(nodeAddr string, item resource.Resource),
) error {
	if c == nil || c.C == nil {
		return nil
	}

	if len(targets) == 0 {
		targets = c.Endpoints
	}

	var lastErr error
	successCount := 0

	for _, target := range targets {
		nodeCtx := talosclient.WithNode(ctx, target)

		list, err := c.C.COSI.List(nodeCtx, md)
		if err != nil {
			lastErr = err
			continue
		}

		successCount++

		for _, item := range list.Items {
			fn(target, item)
		}
	}

	if successCount == 0 && lastErr != nil {
		return lastErr
	}

	return nil
}

// listAllNodes discovers all cluster nodes via NodeTargets, then queries
// each one for the given COSI resource. The fn callback receives resolved
// hostnames (not raw IPs). Use this for per-node resources like addresses,
// routes, storage. Do NOT use for cluster.GetMembers (circular dependency).
func listAllNodes(
	ctx context.Context,
	c *talos.Client,
	md resource.Metadata,
	fn func(nodeHostname string, item resource.Resource),
) error {
	targets, resolve := cluster.NodeTargets(ctx, c)
	return listPerNodeTargets(ctx, c, targets, md, func(addr string, item resource.Resource) {
		fn(resolve(addr), item)
	})
}
