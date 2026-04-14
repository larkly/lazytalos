package clustergrid

import (
	"context"
	"time"

	"charm.land/bubbletea/v2"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// perClusterDeadline bounds the whole fetch-per-cluster pipeline (dial +
// members + services). Kept tight so the grid fails unreachable clusters
// fast rather than stalling a refresh behind one dead site.
const perClusterDeadline = 8 * time.Second

// fetchCluster returns a tea.Cmd that dials (or reuses) the given context
// and loads the minimum data set needed to render its card. `gen` is the
// dispatch generation: later handlers drop messages whose gen doesn't match
// the current grid generation so stale in-flight fetches don't overwrite
// fresh ones on refresh.
func (m Model) fetchCluster(contextName string, gen int) tea.Cmd {
	talosconfig := m.talosconfig
	activeCtx := m.activeCtx
	active := m.activeClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), perClusterDeadline)
		defer cancel()

		var (
			client *talos.Client
			reused bool
		)
		if active != nil && activeCtx != "" && contextName == activeCtx {
			client = active
			reused = true
		} else {
			c, err := talos.Connect(ctx, talosconfig, contextName)
			if err != nil {
				shared.Debugf("[clustergrid] connect %q: %v", contextName, err)
				return CardErrMsg{contextName: contextName, err: err, gen: gen}
			}
			client = c
		}

		nodes, err := cluster.GetMembers(ctx, client)
		if err != nil && len(nodes) == 0 {
			shared.Debugf("[clustergrid] GetMembers %q: %v", contextName, err)
			return CardErrMsg{
				contextName: contextName,
				client:      client,
				reused:      reused,
				err:         err,
				gen:         gen,
			}
		}

		addrs, resolve := cluster.TargetsFromMembers(nodes)
		svcByNode, _ := cluster.ListServicesByNode(ctx, client, addrs, resolve)
		summary := buildSummary(contextName, nodes, svcByNode)

		return CardReadyMsg{
			contextName: contextName,
			summary:     summary,
			client:      client,
			reused:      reused,
			gen:         gen,
		}
	}
}

// buildSummary computes a clusterSummary from members + service data.
func buildSummary(contextName string, nodes []cluster.NodeInfo, svcByNode map[string][]shared.ServiceRow) clusterSummary {
	s := clusterSummary{
		ContextName:    contextName,
		Nodes:          nodes,
		ServicesByNode: svcByNode,
		FetchedAt:      time.Now(),
	}
	hasAnyData := len(svcByNode) > 0
	for _, n := range nodes {
		if n.IsControlPlane() {
			s.CPCount++
		} else {
			s.WorkerCount++
		}
		if s.TalosVersion == "" && n.TalosVersion != "" {
			s.TalosVersion = n.TalosVersion
		}
		svcs, ok := svcByNode[n.Hostname]
		if ok {
			for _, sv := range svcs {
				if sv.Health == shared.HealthFailed {
					s.FailedCount++
					break
				}
			}
		} else if hasAnyData {
			s.UnreachableCount++
		}
	}
	return s
}
