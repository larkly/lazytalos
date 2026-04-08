package resources

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	perfres "github.com/siderolabs/talos/pkg/machinery/resources/perf"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// CPUStats holds CPU usage and uptime for a single node.
type CPUStats struct {
	NodeHostname string
	UsagePercent float64 // 0.0–1.0
	BootTime     time.Time
}

// ListCPUStats returns CPU usage and uptime for all nodes.
func ListCPUStats(ctx context.Context, c *talos.Client) ([]CPUStats, error) {
	md := resource.NewMetadata(
		perfres.NamespaceName,
		perfres.CPUType,
		"",
		resource.VersionUndefined,
	)

	var results []CPUStats

	err := listPerNode(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*perfres.CPU)
		if !ok {
			shared.Debugf("[resources] unexpected CPU type: %T", item)
			return
		}

		spec := r.TypedSpec()
		total := spec.CPUTotal
		allTime := total.User + total.Nice + total.System + total.Idle +
			total.Iowait + total.Irq + total.SoftIrq + total.Steal +
			total.Guest + total.GuestNice

		var usage float64
		if allTime > 0 {
			usage = (allTime - total.Idle) / allTime
		}

		bootTime := item.Metadata().Created()

		results = append(results, CPUStats{
			NodeHostname: nodeAddr,
			UsagePercent: usage,
			BootTime:     bootTime,
		})
	})

	return results, err
}
