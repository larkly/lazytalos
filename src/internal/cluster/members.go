// Package cluster provides cluster member listing and health aggregation.
package cluster

import (
	"context"
	"regexp"
	"sort"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	clusterres "github.com/siderolabs/talos/pkg/machinery/resources/cluster"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// lastKnownNodes caches node info by address so that when a node leaves
// the cluster (shut down), we can still display its hostname and type.
var (
	lastKnownNodes  = make(map[string]NodeInfo)
	lastKnownNodesMu sync.Mutex
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

	// Query each endpoint for member resources. Member resources are replicated
	// so any single endpoint has the full list. We try each endpoint in case
	// one is unreachable. The talosconfig Nodes list may include unreachable
	// nodes, so we explicitly target endpoints (CPs) only.
	var list *resource.List
	var lastErr error
	for _, ep := range c.Endpoints {
		epCtx := talosclient.WithNode(ctx, ep)
		l, err := c.C.COSI.List(epCtx, memberKind)
		if err != nil {
			lastErr = err
			continue
		}
		list = &l
		break
	}
	if list == nil {
		shared.Debugf("[cluster] COSI List members error: %v", lastErr)
		return nil, lastErr
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

	// Cache live node info by address for future lookups
	knownAddrs := make(map[string]bool)
	lastKnownNodesMu.Lock()
	for _, n := range nodes {
		for _, a := range n.Addresses {
			knownAddrs[a] = true
			lastKnownNodes[a] = n
		}
	}

	// Include configured nodes (from talosconfig) that are not in the member
	// list — these are nodes that have been shut down or removed from the
	// cluster but still appear in the operator's config.
	for _, cfgNode := range c.Nodes {
		if knownAddrs[cfgNode] {
			continue
		}
		// Use cached info if available (preserves hostname/type from when node was alive)
		if cached, ok := lastKnownNodes[cfgNode]; ok {
			cached.Healthy = false
			nodes = append(nodes, cached)
			continue
		}
		nodes = append(nodes, NodeInfo{
			Hostname:    cfgNode,
			Addresses:   []string{cfgNode},
			MachineType: "unknown",
			Healthy:     false,
		})
	}
	lastKnownNodesMu.Unlock()

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Hostname < nodes[j].Hostname
	})

	return nodes, nil
}
