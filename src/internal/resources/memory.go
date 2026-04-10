package resources

import (
	"context"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// ListMemStats returns memory statistics for all reachable nodes.
// Callers should set a timeout on ctx if desired.
func ListMemStats(ctx context.Context, c *talos.Client) ([]shared.MemStats, error) {
	if c == nil || c.C == nil {
		return nil, nil
	}

	targets, resolve := cluster.NodeTargets(ctx, c)
	nodeCtx := talosclient.WithNodes(ctx, targets...)
	resp, err := c.C.Memory(nodeCtx)

	if resp == nil {
		return nil, err
	}

	var results []shared.MemStats
	for _, nodeMsg := range resp.GetMessages() {
		if nodeMsg.GetMetadata() == nil || nodeMsg.GetMeminfo() == nil {
			continue
		}
		hostname := resolve(nodeMsg.GetMetadata().GetHostname())
		if hostname == "" {
			continue
		}
		results = append(results, shared.MemStats{
			NodeHostname: hostname,
			TotalKB:      nodeMsg.GetMeminfo().GetMemtotal(),
			AvailableKB:  nodeMsg.GetMeminfo().GetMemavailable(),
		})
	}
	return results, nil
}
