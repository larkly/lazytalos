package resources

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// BlockDevice holds information about a physical disk from a single node.
// Uses the block.Disk resource (not block.Device) since Disk carries size and
// bus-path metadata that is relevant for display purposes.
type BlockDevice struct {
	NodeHostname string
	Name         string
	DevType      string // transport type (e.g. "sata", "nvme", "virtio")
	BusPath      string
	Size         uint64
}

// DiscoveredVolume holds information about a discovered volume from a single node.
type DiscoveredVolume struct {
	NodeHostname string
	Name         string
	FSType       string // "type" field in the spec
	Label        string
	UUID         string
	Size         uint64
}

// VolumeStatus holds volume status information from a single node.
type VolumeStatus struct {
	NodeHostname string
	Name         string
	Phase        string
	MountSpec    string // TargetPath from the mount spec
}

// ListBlockDevices returns all block.Disk resources (physical disks) across all nodes.
func ListBlockDevices(ctx context.Context, c *talos.Client) ([]BlockDevice, error) {
	md := resource.NewMetadata(
		blockres.NamespaceName,
		blockres.DiskType,
		"",
		resource.VersionUndefined,
	)

	var results []BlockDevice

	err := listPerNode(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*blockres.Disk)
		if !ok {
			shared.Debugf("[resources] unexpected Disk type: %T", item)
			return
		}

		spec := r.TypedSpec()
		results = append(results, BlockDevice{
			NodeHostname: nodeAddr,
			Name:         r.Metadata().ID(),
			DevType:      spec.Transport,
			BusPath:      spec.BusPath,
			Size:         spec.Size,
		})
	})

	return results, err
}

// ListDiscoveredVolumes returns all DiscoveredVolume resources across all nodes.
func ListDiscoveredVolumes(ctx context.Context, c *talos.Client) ([]DiscoveredVolume, error) {
	md := resource.NewMetadata(
		blockres.NamespaceName,
		blockres.DiscoveredVolumeType,
		"",
		resource.VersionUndefined,
	)

	var results []DiscoveredVolume

	err := listPerNode(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*blockres.DiscoveredVolume)
		if !ok {
			shared.Debugf("[resources] unexpected DiscoveredVolume type: %T", item)
			return
		}

		spec := r.TypedSpec()
		results = append(results, DiscoveredVolume{
			NodeHostname: nodeAddr,
			Name:         spec.Name,
			FSType:       spec.Type,
			Label:        spec.Label,
			UUID:         spec.UUID,
			Size:         spec.Size,
		})
	})

	return results, err
}

// ListVolumeStatuses returns all VolumeStatus resources across all nodes.
func ListVolumeStatuses(ctx context.Context, c *talos.Client) ([]VolumeStatus, error) {
	md := resource.NewMetadata(
		blockres.NamespaceName,
		blockres.VolumeStatusType,
		"",
		resource.VersionUndefined,
	)

	var results []VolumeStatus

	err := listPerNode(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*blockres.VolumeStatus)
		if !ok {
			shared.Debugf("[resources] unexpected VolumeStatus type: %T", item)
			return
		}

		spec := r.TypedSpec()
		results = append(results, VolumeStatus{
			NodeHostname: nodeAddr,
			Name:         r.Metadata().ID(),
			Phase:        spec.Phase.String(),
			MountSpec:    spec.MountSpec.TargetPath,
		})
	})

	return results, err
}
