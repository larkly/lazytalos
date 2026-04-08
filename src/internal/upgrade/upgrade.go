package upgrade

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
)

// NodePhase represents the upgrade phase of a single node.
type NodePhase int

const (
	NodePhasePending NodePhase = iota
	NodePhaseUpgrading
	NodePhaseWaitingHealth
	NodePhaseDone
	NodePhaseError
)

// NodeState holds the per-node upgrade tracking state.
type NodeState struct {
	Hostname      string
	Address       string // routable IP for WithNodes; falls back to Hostname
	Phase         NodePhase
	ErrMsg        string
	StartedAt     time.Time
	FinishedAt    time.Time
	HealthRetries int
}

// Options holds the parameters for a cluster upgrade.
type Options struct {
	Image    string
	Preserve bool
	Stage    bool
}

// State holds the full upgrade state machine.
type State struct {
	Nodes   []NodeState
	Options Options
	Active  int  // index of node currently upgrading, -1 = not started
	Paused  bool
	Aborted bool
}

// NewState creates an upgrade State with workers first, then control planes.
func NewState(nodes []cluster.NodeInfo, opts Options) State {
	var workers, cps []cluster.NodeInfo
	for _, n := range nodes {
		if n.IsControlPlane() {
			cps = append(cps, n)
		} else {
			workers = append(workers, n)
		}
	}
	ordered := append(workers, cps...)
	states := make([]NodeState, len(ordered))
	for i, n := range ordered {
		addr := n.Hostname
		if len(n.Addresses) > 0 {
			addr = n.Addresses[0]
		}
		states[i] = NodeState{Hostname: n.Hostname, Address: addr, Phase: NodePhasePending}
	}
	return State{Nodes: states, Options: opts, Active: -1}
}

// StartNode issues the Upgrade RPC for State.Nodes[idx].
func StartNode(ctx context.Context, c *talos.Client, s State, idx int) tea.Cmd {
	return func() tea.Msg {
		if c == nil || c.C == nil {
			return shared.NodeUpgradeErrMsg{Index: idx, Err: fmt.Errorf("no client")}
		}
		target := s.Nodes[idx].Address
		nodeCtx := talosclient.WithNodes(ctx, target)
		tCtx, cancel := context.WithTimeout(nodeCtx, 10*time.Minute)
		defer cancel()
		_, err := c.C.Upgrade(tCtx, s.Options.Image, s.Options.Stage, s.Options.Preserve)
		if err != nil {
			return shared.NodeUpgradeErrMsg{Index: idx, Err: err}
		}
		return shared.NodeUpgradedMsg{Index: idx}
	}
}

// PollHealth checks if the node is back up by calling Version.
func PollHealth(ctx context.Context, c *talos.Client, idx int, address string) tea.Cmd {
	return func() tea.Msg {
		if c == nil || c.C == nil {
			return shared.NodeHealthErrMsg{Index: idx, Err: fmt.Errorf("no client")}
		}
		nodeCtx := talosclient.WithNodes(ctx, address)
		tCtx, cancel := context.WithTimeout(nodeCtx, 15*time.Second)
		defer cancel()
		_, err := c.C.Version(tCtx)
		if err != nil {
			return shared.NodeHealthErrMsg{Index: idx, Err: err}
		}
		return shared.NodeHealthyMsg{Index: idx}
	}
}
