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

func TestTypedConfirm(t *testing.T) {
	m := NewTypedConfirm("remove etcd member", "tnn3-demo-cp-1", "tnn3-demo-cp-1")

	// Ctrl+S with no input typed — should be ignored (no cmd returned).
	ctrlS := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	m2, cmd := m.Update(ctrlS)
	if cmd != nil {
		t.Error("expected no command when typedInput does not match RequiredInput")
	}

	// Type the required string character by character.
	for _, ch := range "tnn3-demo-cp-1" {
		keyMsg := tea.KeyPressMsg{Code: ch, Text: string(ch)}
		m2, _ = m2.Update(keyMsg)
	}

	// Ctrl+S now — should confirm.
	m2, cmd = m2.Update(ctrlS)
	if cmd == nil {
		t.Fatal("expected a command after typing required input and pressing ctrl+s")
	}
	result := cmd()
	action, ok := result.(ConfirmAction)
	if !ok {
		t.Fatalf("expected ConfirmAction, got %T", result)
	}
	if !action.Confirm {
		t.Error("expected Confirm=true after typing required input")
	}
	if action.Node != "tnn3-demo-cp-1" {
		t.Errorf("expected node=tnn3-demo-cp-1, got %q", action.Node)
	}

	// Test backspace: type one extra char then backspace — should not confirm.
	m3 := NewTypedConfirm("remove etcd member", "tnn3-demo-cp-1", "abc")
	for _, ch := range "abcd" {
		keyMsg := tea.KeyPressMsg{Code: ch, Text: string(ch)}
		m3, _ = m3.Update(keyMsg)
	}
	// Backspace removes 'd', leaving "abc".
	bsMsg := tea.KeyPressMsg{Code: tea.KeyBackspace}
	m3, _ = m3.Update(bsMsg)
	// Now typedInput should be "abc" — ctrl+s should confirm.
	m3, cmd = m3.Update(ctrlS)
	if cmd == nil {
		t.Fatal("expected a command after backspace correction and ctrl+s")
	}
	result = cmd()
	action, ok = result.(ConfirmAction)
	if !ok {
		t.Fatalf("expected ConfirmAction, got %T", result)
	}
	if !action.Confirm {
		t.Error("expected Confirm=true after backspace correction")
	}

	// Esc should cancel even in typed mode.
	m4 := NewTypedConfirm("remove etcd member", "node-x", "node-x")
	escMsg := tea.KeyPressMsg{Code: tea.KeyEsc}
	_, cmd = m4.Update(escMsg)
	if cmd == nil {
		t.Fatal("expected a command after esc in typed mode")
	}
	result = cmd()
	action, ok = result.(ConfirmAction)
	if !ok {
		t.Fatalf("expected ConfirmAction, got %T", result)
	}
	if action.Confirm {
		t.Error("expected Confirm=false after esc in typed mode")
	}
}
