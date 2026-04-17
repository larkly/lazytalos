package clustergrid

import (
	"context"
	"sync"
	"time"

	"charm.land/bubbletea/v2"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/resources"
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

		// Fetch memory + CPU in parallel so the card dots can reflect resource
		// pressure like the dashboard node matrix does. Failures here degrade
		// gracefully: an empty map means "no signal" and the dot falls back to
		// service-only logic.
		var (
			memByNode map[string]shared.MemStats
			cpuByNode map[string]resources.CPUStats
			wg        sync.WaitGroup
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			stats, err := resources.ListMemStats(ctx, client)
			if err != nil {
				shared.Debugf("[clustergrid] ListMemStats %q: %v", contextName, err)
			}
			memByNode = make(map[string]shared.MemStats, len(stats))
			for _, s := range stats {
				memByNode[s.NodeHostname] = s
			}
		}()
		go func() {
			defer wg.Done()
			stats, err := resources.ListCPUStats(ctx, client)
			if err != nil {
				shared.Debugf("[clustergrid] ListCPUStats %q: %v", contextName, err)
			}
			cpuByNode = make(map[string]resources.CPUStats, len(stats))
			for _, s := range stats {
				cpuByNode[s.NodeHostname] = s
			}
		}()
		wg.Wait()

		summary := buildSummary(contextName, nodes, svcByNode, memByNode, cpuByNode)

		return CardReadyMsg{
			contextName: contextName,
			summary:     summary,
			client:      client,
			reused:      reused,
			gen:         gen,
		}
	}
}

// buildSummary computes a clusterSummary from members + service + resource data.
func buildSummary(
	contextName string,
	nodes []cluster.NodeInfo,
	svcByNode map[string][]shared.ServiceRow,
	memByNode map[string]shared.MemStats,
	cpuByNode map[string]resources.CPUStats,
) clusterSummary {
	s := clusterSummary{
		ContextName:    contextName,
		Nodes:          nodes,
		ServicesByNode: svcByNode,
		MemoryByNode:   memByNode,
		CPUByNode:      cpuByNode,
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
		if !ok {
			if hasAnyData {
				s.UnreachableCount++
			}
			continue
		}
		failed := false
		for _, sv := range svcs {
			if sv.Health == shared.HealthFailed {
				failed = true
				break
			}
		}
		if !failed {
			// Memory pressure above the critical threshold renders as an error
			// dot in both the grid and the dashboard node matrix; count it
			// here so the card's "N failed" summary stays honest about dots.
			if shared.MemUsedPct(memByNode[n.Hostname]) > shared.MemCriticalPct {
				failed = true
			}
		}
		if failed {
			s.FailedCount++
		}
	}
	return s
}
