package cluster

import (
	"context"

	"github.com/larkly/lazytalos/internal/talos"
)

// NodeInfo describes a single cluster member.
type NodeInfo struct {
	Hostname     string
	MachineType  string
	Addresses    []string
	TalosVersion string
	Healthy      bool
}

// GetMembers returns the list of cluster members.
func GetMembers(_ context.Context, _ *talos.Client) ([]NodeInfo, error) {
	return nil, nil // stub
}
