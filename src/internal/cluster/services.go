package cluster

import (
	"context"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// TargetsFromMembers builds (addrs, resolve) from an already-fetched member
// list without re-calling GetMembers or running a reachability probe. Use
// this instead of NodeTargets when the caller already has members and
// tolerates unreachable nodes (e.g. dashboards that render offline nodes
// explicitly rather than hiding them).
func TargetsFromMembers(members []NodeInfo) (addrs []string, resolve func(string) string) {
	identity := func(s string) string { return s }
	if len(members) == 0 {
		return nil, identity
	}
	addrToHost := make(map[string]string, len(members)*2)
	addrs = make([]string, 0, len(members))
	for _, m := range members {
		for _, a := range m.Addresses {
			addrToHost[a] = m.Hostname
		}
		addrToHost[m.Hostname] = m.Hostname
		if len(m.Addresses) > 0 {
			addrs = append(addrs, m.Addresses[0])
		}
	}
	resolve = func(s string) string {
		if h, ok := addrToHost[s]; ok {
			return h
		}
		return s
	}
	return addrs, resolve
}

// ListServicesByNode runs a multi-node ServiceList against `addrs` and
// groups the results by canonical hostname (via `resolve`). Returns a
// partial map even on per-node errors so healthy nodes still render.
func ListServicesByNode(
	ctx context.Context,
	client *talos.Client,
	addrs []string,
	resolve func(string) string,
) (map[string][]shared.ServiceRow, error) {
	byNode := make(map[string][]shared.ServiceRow)
	if client == nil || client.C == nil || len(addrs) == 0 {
		return byNode, nil
	}
	nodeCtx := talosclient.WithNodes(ctx, addrs...)
	resp, err := client.C.ServiceList(nodeCtx)
	if resp == nil {
		return byNode, err
	}
	for _, nodeMsg := range resp.GetMessages() {
		if nodeMsg.GetMetadata() == nil {
			continue
		}
		hostname := resolve(nodeMsg.GetMetadata().GetHostname())
		if hostname == "" {
			continue
		}
		var svcs []shared.ServiceRow
		for _, svc := range nodeMsg.GetServices() {
			svcs = append(svcs, shared.ServiceRow{
				ServiceID: svc.GetId(),
				State:     svc.GetState(),
				Health:    shared.ClassifyServiceHealth(svc.GetHealth()),
			})
		}
		byNode[hostname] = svcs
	}
	return byNode, err
}
