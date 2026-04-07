package upgrade

import (
	"testing"

	"github.com/larkly/lazytalos/internal/cluster"
)

func TestNewState_WorkersFirst(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
		{Hostname: "w-1", MachineType: "worker"},
		{Hostname: "cp-2", MachineType: "controlplane"},
		{Hostname: "w-2", MachineType: "worker"},
	}
	s := NewState(nodes, Options{Image: "img"})
	if s.Nodes[0].Hostname != "w-1" || s.Nodes[1].Hostname != "w-2" {
		t.Errorf("expected workers first, got %v, %v", s.Nodes[0].Hostname, s.Nodes[1].Hostname)
	}
}

func TestNewState_CPsOnly(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
		{Hostname: "cp-2", MachineType: "controlplane"},
	}
	s := NewState(nodes, Options{})
	if len(s.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(s.Nodes))
	}
	for _, n := range s.Nodes {
		if n.Phase != NodePhasePending {
			t.Errorf("expected pending phase")
		}
	}
}

func TestNodePhaseConstants(t *testing.T) {
	phases := []NodePhase{NodePhasePending, NodePhaseUpgrading, NodePhaseWaitingHealth, NodePhaseDone, NodePhaseError}
	seen := map[NodePhase]bool{}
	for _, p := range phases {
		if seen[p] {
			t.Errorf("duplicate phase value %d", p)
		}
		seen[p] = true
	}
}
