// Package node provides helpers for fetching per-node data from a Talos cluster.
package node

import (
	"context"
	"strings"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/api/common"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/talos"
)

// Container represents a single container on a Talos node.
type Container struct {
	NodeHostname string
	Namespace    string
	Name         string
	Image        string // short form (last segment of full image)
	FullImage    string
	State        string
	PID          uint32
	Status       string
}

// ListContainers fetches all containers across all nodes in the client's context.
// It queries both the "system" and "k8s.io" containerd namespaces.
func ListContainers(ctx context.Context, c *talos.Client) ([]Container, error) {
	if c == nil || c.C == nil {
		return nil, nil
	}

	targets, resolve := nodeTargets(ctx, c)
	nodeCtx := talosclient.WithNodes(ctx, targets...)
	var containers []Container
	for _, ns := range []string{"system", "k8s.io"} {
		resp, err := c.C.Containers(nodeCtx, ns, common.ContainerDriver_CONTAINERD)
		if err != nil {
			continue
		}
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			hostname := resolve(nodeMsg.GetMetadata().GetHostname())
			if hostname == "" {
				continue
			}
			for _, ci := range nodeMsg.GetContainers() {
				name := ci.GetName()
				if name == "" {
					name = ci.GetId()
				}
				fullImage := ci.GetImage()
				containers = append(containers, Container{
					NodeHostname: hostname,
					Namespace:    ns,
					Name:         name,
					Image:        shortImage(fullImage),
					FullImage:    fullImage,
					State:        ci.GetStatus(),
					PID:          ci.GetPid(),
					Status:       ci.GetStatus(),
				})
			}
		}
	}
	return containers, nil
}

func nodeTargets(ctx context.Context, c *talos.Client) (addrs []string, resolveHostname func(string) string) {
	identity := func(s string) string { return s }
	members, err := cluster.GetMembers(ctx, c)
	if err != nil || len(members) == 0 {
		return c.Endpoints, identity
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
		return c.Endpoints, identity
	}
	return addrs, func(s string) string {
		if h, ok := addrToHost[s]; ok {
			return h
		}
		return s
	}
}

// shortImage extracts the last path segment from a full image reference and
// strips any digest suffix (e.g. "@sha256:...").
func shortImage(image string) string {
	// Strip digest
	if idx := strings.Index(image, "@"); idx != -1 {
		image = image[:idx]
	}
	// Take last path segment
	if idx := strings.LastIndex(image, "/"); idx != -1 {
		image = image[idx+1:]
	}
	return image
}
