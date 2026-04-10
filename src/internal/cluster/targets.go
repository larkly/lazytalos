package cluster

import (
	"context"
	"time"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/talos"
)

// NodeTargets discovers all cluster nodes and returns their addresses plus
// a resolver function that maps any address/IP back to the canonical hostname.
// Unreachable nodes are excluded from the address list to prevent multi-node
// API calls from failing entirely when one node is down.
// Falls back to client.Endpoints if member discovery fails.
func NodeTargets(ctx context.Context, client *talos.Client) (addrs []string, resolveHostname func(string) string) {
	identity := func(s string) string { return s }
	if client == nil {
		return nil, identity
	}
	members, err := GetMembers(ctx, client)
	if err != nil || len(members) == 0 {
		return client.Endpoints, identity
	}

	// Build resolver map from all members (including unreachable)
	addrToHost := make(map[string]string)
	var allAddrs []string
	for _, m := range members {
		for _, a := range m.Addresses {
			addrToHost[a] = m.Hostname
		}
		addrToHost[m.Hostname] = m.Hostname
		if len(m.Addresses) > 0 {
			allAddrs = append(allAddrs, m.Addresses[0])
		}
	}
	if len(allAddrs) == 0 {
		return client.Endpoints, identity
	}

	resolve := func(s string) string {
		if h, ok := addrToHost[s]; ok {
			return h
		}
		return s
	}

	// Quick reachability probe: single multi-node Version call
	// Only include nodes that respond to avoid failing entire API calls
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	nodeCtx := talosclient.WithNodes(probeCtx, allAddrs...)
	resp, _ := client.C.Version(nodeCtx)
	if resp == nil {
		// Probe failed entirely — fall back to all addresses
		return allAddrs, resolve
	}

	reachable := make(map[string]bool)
	for _, msg := range resp.GetMessages() {
		if msg.GetMetadata() == nil {
			continue
		}
		h := msg.GetMetadata().GetHostname()
		reachable[h] = true
	}

	// Filter to only reachable nodes
	var filtered []string
	for _, addr := range allAddrs {
		hostname := resolve(addr)
		if reachable[hostname] || reachable[addr] {
			filtered = append(filtered, addr)
		}
	}
	if len(filtered) == 0 {
		return allAddrs, resolve // don't return empty if probe was flaky
	}
	return filtered, resolve
}
