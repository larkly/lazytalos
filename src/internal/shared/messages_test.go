package shared

import "testing"

func TestNewMessageTypes(t *testing.T) {
	t.Run("ContainerLogsRequestMsg", func(t *testing.T) {
		msg := ContainerLogsRequestMsg{
			Node:        "controlplane-1",
			Namespace:   "k8s.io",
			ContainerID: "abc123",
		}
		if msg.Node != "controlplane-1" {
			t.Errorf("Node: got %q, want %q", msg.Node, "controlplane-1")
		}
		if msg.Namespace != "k8s.io" {
			t.Errorf("Namespace: got %q, want %q", msg.Namespace, "k8s.io")
		}
		if msg.ContainerID != "abc123" {
			t.Errorf("ContainerID: got %q, want %q", msg.ContainerID, "abc123")
		}
	})

	t.Run("ConfigEditRequestMsg", func(t *testing.T) {
		msg := ConfigEditRequestMsg{
			Node: "worker-0",
		}
		if msg.Node != "worker-0" {
			t.Errorf("Node: got %q, want %q", msg.Node, "worker-0")
		}
	})

	t.Run("EtcdMemberRemoveRequestMsg", func(t *testing.T) {
		msg := EtcdMemberRemoveRequestMsg{
			Node:     "controlplane-2",
			MemberID: 12345678901234,
		}
		if msg.Node != "controlplane-2" {
			t.Errorf("Node: got %q, want %q", msg.Node, "controlplane-2")
		}
		if msg.MemberID != 12345678901234 {
			t.Errorf("MemberID: got %d, want %d", msg.MemberID, uint64(12345678901234))
		}
	})
}
