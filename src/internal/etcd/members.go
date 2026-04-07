// Package etcd provides helpers for querying and managing the etcd cluster
// via the Talos machinery gRPC client.
package etcd

import (
	"context"
	"fmt"

	"github.com/larkly/lazytalos/internal/talos"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
)

// Member represents an etcd cluster member as seen from a control plane node.
type Member struct {
	NodeHostname string   // which CP node provided this data
	MemberID     uint64
	Hostname     string
	PeerAddrs    []string
	ClientAddrs  []string
	IsLearner    bool
	IsLeader     bool // always false for MVP; Talos API does not expose leader per-member
}

// ListMembers fetches the etcd member list from all control plane nodes and
// deduplicates results by MemberID (keeping the first occurrence).
func ListMembers(ctx context.Context, c *talos.Client) ([]Member, error) {
	resp, err := c.C.EtcdMemberList(ctx, &machine.EtcdMemberListRequest{})
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}

	seen := make(map[uint64]struct{})
	var members []Member

	for _, msg := range resp.Messages {
		nodeHostname := ""
		if msg.Metadata != nil {
			nodeHostname = msg.Metadata.GetHostname()
		}
		for _, m := range msg.Members {
			if _, exists := seen[m.GetId()]; exists {
				continue
			}
			seen[m.GetId()] = struct{}{}
			members = append(members, Member{
				NodeHostname: nodeHostname,
				MemberID:     m.GetId(),
				Hostname:     m.GetHostname(),
				PeerAddrs:    m.GetPeerUrls(),
				ClientAddrs:  m.GetClientUrls(),
				IsLearner:    m.GetIsLearner(),
				IsLeader:     false,
			})
		}
	}

	return members, nil
}

// RemoveMemberByID removes an etcd member by its numeric ID.
// node is the control-plane node hostname to target for the RPC.
func RemoveMemberByID(ctx context.Context, c *talos.Client, node string, memberID uint64) error {
	nodeCtx := talosclient.WithNodes(ctx, node)
	err := c.C.EtcdRemoveMemberByID(nodeCtx, &machine.EtcdRemoveMemberByIDRequest{
		MemberId: memberID,
	})
	if err != nil {
		return fmt.Errorf("etcd remove member %d via %s: %w", memberID, node, err)
	}
	return nil
}
