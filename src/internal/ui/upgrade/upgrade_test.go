package upgrade

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/larkly/lazytalos/internal/cluster"
)

func testNodes() []cluster.NodeInfo {
	return []cluster.NodeInfo{
		{Hostname: "worker-1", MachineType: "worker"},
		{Hostname: "worker-2", MachineType: "worker"},
		{Hostname: "cp-1", MachineType: "controlplane"},
	}
}

func TestNew(t *testing.T) {
	nodes := testNodes()
	m := New(nil, "mycluster", nodes, 80, 40)
	if len(m.nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(m.nodes))
	}
	if m.step != 0 {
		t.Fatalf("expected step 0, got %d", m.step)
	}
	if len(m.nodeCheckboxes) != 3 {
		t.Fatalf("expected 3 checkboxes, got %d", len(m.nodeCheckboxes))
	}
	for i, c := range m.nodeCheckboxes {
		if !c {
			t.Fatalf("expected checkbox %d to be true", i)
		}
	}
}

func TestStepForward(t *testing.T) {
	m := New(nil, "mycluster", testNodes(), 80, 40)
	// All nodes selected, press Enter to go to step 1.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.step != 1 {
		t.Fatalf("expected step 1, got %d", m.step)
	}
}

func TestStepBack(t *testing.T) {
	m := New(nil, "mycluster", testNodes(), 80, 40)
	// Advance to step 1.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.step != 1 {
		t.Fatalf("expected step 1, got %d", m.step)
	}
	// Press Esc to go back to step 0.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.step != 0 {
		t.Fatalf("expected step 0, got %d", m.step)
	}
}

func TestEscOnStep0(t *testing.T) {
	m := New(nil, "mycluster", testNodes(), 80, 40)
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a command (ClosedMsg)")
	}
	msg := cmd()
	closed, ok := msg.(ClosedMsg)
	if !ok {
		t.Fatalf("expected ClosedMsg, got %T", msg)
	}
	if closed.Completed {
		t.Fatal("expected Completed=false")
	}
}

func TestTypedConfirmRejectsWrong(t *testing.T) {
	m := New(nil, "mycluster", testNodes(), 80, 40)
	// Fast-forward to step 4.
	m.step = 4
	m.confirmInput.SetValue("wrong")
	m.confirmInput.Focus()

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.step != 4 {
		t.Fatalf("expected to stay on step 4, got %d", m.step)
	}
	if m.errMsg == "" {
		t.Fatal("expected error message")
	}
}

func TestTypedConfirmAcceptsCorrect(t *testing.T) {
	m := New(nil, "mycluster", testNodes(), 80, 40)

	// Build state as if we went through steps 0-3.
	m.step = 4
	m.confirmInput.SetValue("mycluster")
	m.confirmInput.Focus()

	// We need a state for step 5 to work, so pre-populate via step 2->3 transition logic.
	m.imageInput.SetValue("ghcr.io/siderolabs/installer:v1.9.0")

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.step != 5 {
		t.Fatalf("expected step 5, got %d", m.step)
	}
}

func TestNodeSelection(t *testing.T) {
	m := New(nil, "mycluster", testNodes(), 80, 40)
	if !m.nodeCheckboxes[0] {
		t.Fatal("expected node 0 checked by default")
	}
	// Press space to toggle node 0 off.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.nodeCheckboxes[0] {
		t.Fatal("expected node 0 unchecked after space")
	}
	// Press space again to toggle back on.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !m.nodeCheckboxes[0] {
		t.Fatal("expected node 0 checked after second space")
	}
}
