package etcd

import (
	"testing"
)

// TestMemberStructCompilation verifies that the Member struct fields compile correctly.
func TestMemberStructCompilation(t *testing.T) {
	m := Member{
		NodeHostname: "cp-1",
		MemberID:     12345,
		Hostname:     "etcd-member-1",
		PeerAddrs:    []string{"http://10.0.0.1:2380"},
		ClientAddrs:  []string{"http://10.0.0.1:2379"},
		IsLearner:    false,
		IsLeader:     false,
	}
	if m.MemberID != 12345 {
		t.Fatalf("unexpected MemberID: %d", m.MemberID)
	}
}

// TestDeduplication verifies the deduplication logic: given a simulated set of
// (MemberID -> Member) entries, duplicate IDs must be collapsed to a single entry.
func TestDeduplication(t *testing.T) {
	// Simulate what ListMembers does internally.
	type rawEntry struct {
		nodeHostname string
		id           uint64
		hostname     string
	}

	// Two CP nodes report the same three members.
	raw := []rawEntry{
		{"cp-1", 1, "etcd-1"},
		{"cp-1", 2, "etcd-2"},
		{"cp-1", 3, "etcd-3"},
		{"cp-2", 1, "etcd-1"}, // duplicate
		{"cp-2", 2, "etcd-2"}, // duplicate
		{"cp-2", 3, "etcd-3"}, // duplicate
	}

	seen := make(map[uint64]struct{})
	var members []Member

	for _, e := range raw {
		if _, exists := seen[e.id]; exists {
			continue
		}
		seen[e.id] = struct{}{}
		members = append(members, Member{
			NodeHostname: e.nodeHostname,
			MemberID:     e.id,
			Hostname:     e.hostname,
		})
	}

	if len(members) != 3 {
		t.Fatalf("expected 3 deduplicated members, got %d", len(members))
	}

	// All deduplicated entries should come from the first node.
	for _, m := range members {
		if m.NodeHostname != "cp-1" {
			t.Errorf("expected NodeHostname cp-1, got %s", m.NodeHostname)
		}
	}
}
