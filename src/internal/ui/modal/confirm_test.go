package modal

import (
	"testing"

	"charm.land/bubbletea/v2"
)

func TestNewBulkConfirmWithCPNode(t *testing.T) {
	nodes := []NodeRef{
		{ID: "node1", Name: "worker-1", IsControlPlane: false},
		{ID: "node2", Name: "controlplane-1", IsControlPlane: true},
	}
	m := NewBulkConfirm("reboot", nodes)
	if !m.HasControlPlane {
		t.Error("expected HasControlPlane to be true when a control plane node is included")
	}
}

func TestNewBulkConfirmWithoutCPNode(t *testing.T) {
	nodes := []NodeRef{
		{ID: "node1", Name: "worker-1", IsControlPlane: false},
		{ID: "node2", Name: "worker-2", IsControlPlane: false},
	}
	m := NewBulkConfirm("reboot", nodes)
	if m.HasControlPlane {
		t.Error("expected HasControlPlane to be false when no control plane nodes are included")
	}
}

func TestConfirmCtrlS(t *testing.T) {
	m := NewConfirm("reboot", "node-1")
	m.focused = 0 // focus on confirm button

	// ctrl+s produces "ctrl+s" string via Key.String()
	msg := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command after ctrl+s, got nil")
	}
	result := cmd()
	action, ok := result.(ConfirmAction)
	if !ok {
		t.Fatalf("expected ConfirmAction, got %T", result)
	}
	if !action.Confirm {
		t.Error("expected Confirm=true after ctrl+s")
	}
	if action.Action != "reboot" {
		t.Errorf("expected action=reboot, got %q", action.Action)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	m := NewConfirm("shutdown", "node-2")

	msg := tea.KeyPressMsg{Code: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command after esc, got nil")
	}
	result := cmd()
	action, ok := result.(ConfirmAction)
	if !ok {
		t.Fatalf("expected ConfirmAction, got %T", result)
	}
	if action.Confirm {
		t.Error("expected Confirm=false after esc")
	}
}
