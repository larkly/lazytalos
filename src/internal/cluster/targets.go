package cluster

import (
	"context"

	"github.com/larkly/lazytalos/internal/talos"
)

// NodeTargets discovers all cluster nodes and returns their addresses plus
// a resolver function that maps any address/IP back to the canonical hostname.
// The Talos API returns IPs (not hostnames) in response metadata, so callers
// must use the resolver to normalize response hostnames to member hostnames.
// Falls back to client.Endpoints if member discovery fails.
func NodeTargets(ctx context.Context, client *talos.Client) (addrs []string, resolveHostname func(string) string) {
	identity := func(s string) string { return s }
	members, err := GetMembers(ctx, client)
	if err != nil || len(members) == 0 {
		return client.Endpoints, identity
	}

	addrToHost := make(map[string]string)
	for _, m := range members {
		for _, a := range m.Addresses {
			addrToHost[a] = m.Hostname
		}
		addrToHost[m.Hostname] = m.Hostname
		if len(m.Addresses) > 0 {
			addrs = append(addrs, m.Addresses[0])
		}
	}
	if len(addrs) == 0 {
		return client.Endpoints, identity
	}
	return addrs, func(s string) string {
		if h, ok := addrToHost[s]; ok {
			return h
		}
		return s
	}
}
