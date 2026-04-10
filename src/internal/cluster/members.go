// Package cluster provides cluster member listing and health aggregation.
package cluster

import (
	"context"
	"regexp"
	"sort"

	"github.com/cosi-project/runtime/pkg/resource"
	clusterres "github.com/siderolabs/talos/pkg/machinery/resources/cluster"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// NodeInfo describes a single cluster member.
type NodeInfo struct {
	Hostname     string
	MachineType  string // "controlplane" or "worker"
	Addresses    []string
	TalosVersion string
	Healthy      bool
}

// IsControlPlane returns true if the node is a control plane node.
func (n NodeInfo) IsControlPlane() bool {
	return n.MachineType == "controlplane" || n.MachineType == "init"
}

// versionRegexp extracts a version from an OperatingSystem string like "Talos (v1.12.4)".
var versionRegexp = regexp.MustCompile(`\(v([^)]+)\)`)

// ParseTalosVersion extracts a version string from the Talos OperatingSystem field.
// The format is typically "Talos (v1.12.4)". Returns the raw string if parsing fails.
func ParseTalosVersion(os string) string {
	if os == "" {
		return ""
	}
	m := versionRegexp.FindStringSubmatch(os)
	if len(m) >= 2 {
		return "v" + m[1]
	}
	return os
}

// GetMembers returns the list of cluster members via the COSI resource API.
// It fetches cluster.Member resources and builds NodeInfo from the spec.
func GetMembers(ctx context.Context, c *talos.Client) ([]NodeInfo, error) {
	if c == nil || c.C == nil {
		return nil, nil
	}

	// List cluster member resources via COSI state.
	memberKind := resource.NewMetadata(
		clusterres.NamespaceName,
		clusterres.MemberType,
		"",
		resource.VersionUndefined,
	)

	list, err := c.C.COSI.List(ctx, memberKind)
	if err != nil {
		shared.Debugf("[cluster] COSI List members error: %v", err)
		return nil, err
	}

	seen := make(map[string]bool)
	var nodes []NodeInfo

	for _, item := range list.Items {
		member, ok := item.(*clusterres.Member)
		if !ok {
			shared.Debugf("[cluster] unexpected resource type: %T", item)
			continue
		}

		spec := member.TypedSpec()
		hostname := spec.Hostname
		if hostname == "" {
			hostname = member.Metadata().ID()
		}

		// Deduplicate: COSI fan-out returns members from each node's perspective.
		if seen[hostname] {
			continue
		}
		seen[hostname] = true

		addrs := make([]string, 0, len(spec.Addresses))
		for _, a := range spec.Addresses {
			addrs = append(addrs, a.String())
		}

		machineType := spec.MachineType.String()

		node := NodeInfo{
			Hostname:     hostname,
			MachineType:  machineType,
			Addresses:    addrs,
			TalosVersion: ParseTalosVersion(spec.OperatingSystem),
			Healthy:      true, // actual health determined by service data presence
		}
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Hostname < nodes[j].Hostname
	})

	return nodes, nil
}
