package resources

import (
	"context"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// AddressStatus holds a network address from a single node.
type AddressStatus struct {
	NodeHostname string
	Interface    string
	Address      string
	Scope        string
	Flags        string
}

// RouteStatus holds a route entry from a single node.
type RouteStatus struct {
	NodeHostname string
	Destination  string
	Gateway      string
	Interface    string
	Metric       uint32
}

// HostnameStatus holds the hostname reported by a single node.
type HostnameStatus struct {
	NodeHostname string
	Hostname     string
}

// DNSUpstream holds a DNS upstream server from a single node.
type DNSUpstream struct {
	NodeHostname string
	Address      string
}

// ListAddresses returns all AddressStatus resources across all nodes.
func ListAddresses(ctx context.Context, c *talos.Client) ([]AddressStatus, error) {
	md := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.AddressStatusType,
		"",
		resource.VersionUndefined,
	)

	var results []AddressStatus

	err := listAllNodes(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*networkres.AddressStatus)
		if !ok {
			shared.Debugf("[resources] unexpected AddressStatus type: %T", item)
			return
		}

		spec := r.TypedSpec()
		results = append(results, AddressStatus{
			NodeHostname: nodeAddr,
			Interface:    spec.LinkName,
			Address:      prefixString(spec.Address),
			Scope:        spec.Scope.String(),
			Flags:        spec.Flags.String(),
		})
	})

	return results, err
}

// ListRoutes returns all RouteStatus resources across all nodes.
func ListRoutes(ctx context.Context, c *talos.Client) ([]RouteStatus, error) {
	md := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.RouteStatusType,
		"",
		resource.VersionUndefined,
	)

	var results []RouteStatus

	err := listAllNodes(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*networkres.RouteStatus)
		if !ok {
			shared.Debugf("[resources] unexpected RouteStatus type: %T", item)
			return
		}

		spec := r.TypedSpec()
		results = append(results, RouteStatus{
			NodeHostname: nodeAddr,
			Destination:  prefixString(spec.Destination),
			Gateway:      addrString(spec.Gateway),
			Interface:    spec.OutLinkName,
			Metric:       spec.Priority,
		})
	})

	return results, err
}

// ListHostnames returns the HostnameStatus for each node.
func ListHostnames(ctx context.Context, c *talos.Client) ([]HostnameStatus, error) {
	md := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.HostnameStatusType,
		"",
		resource.VersionUndefined,
	)

	var results []HostnameStatus

	err := listAllNodes(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*networkres.HostnameStatus)
		if !ok {
			shared.Debugf("[resources] unexpected HostnameStatus type: %T", item)
			return
		}

		spec := r.TypedSpec()
		results = append(results, HostnameStatus{
			NodeHostname: nodeAddr,
			Hostname:     spec.Hostname,
		})
	})

	return results, err
}

// ListDNSUpstreams returns DNS servers from ResolverStatus resources across all nodes.
func ListDNSUpstreams(ctx context.Context, c *talos.Client) ([]DNSUpstream, error) {
	md := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.ResolverStatusType,
		"",
		resource.VersionUndefined,
	)

	var results []DNSUpstream

	err := listAllNodes(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*networkres.ResolverStatus)
		if !ok {
			shared.Debugf("[resources] unexpected ResolverStatus type: %T", item)
			return
		}

		spec := r.TypedSpec()
		for _, addr := range spec.DNSServers {
			results = append(results, DNSUpstream{
				NodeHostname: nodeAddr,
				Address:      addr.String(),
			})
		}
	})

	if err != nil {
		shared.Debugf("[resources] ResolverStatus list error: %v", err)
		return []DNSUpstream{}, nil
	}

	return results, nil
}

func prefixString(p netip.Prefix) string {
	if !p.IsValid() {
		return ""
	}
	return p.String()
}

func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}
