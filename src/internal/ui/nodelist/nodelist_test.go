package nodelist

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
)

func makeTestNodes() []cluster.NodeInfo {
	return []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane", Addresses: []string{"10.0.0.1"}},
		{Hostname: "cp-2", MachineType: "controlplane", Addresses: []string{"10.0.0.2"}},
		{Hostname: "worker-1", MachineType: "worker", Addresses: []string{"10.0.0.3"}},
	}
}

func TestSpaceToggle(t *testing.T) {
	m := New(nil, 0)
	m.nodes = makeTestNodes()
	m.applyFilter()

	// Select first node
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: ' '}))
	if !m.selected["cp-1"] {
		t.Error("expected cp-1 to be selected after space")
	}

	// Deselect
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: ' '}))
	if m.selected["cp-1"] {
		t.Error("expected cp-1 to be deselected after second space")
	}
}

func TestSelectAll(t *testing.T) {
	m := New(nil, 0)
	m.nodes = makeTestNodes()
	m.applyFilter()
	m.width = 80
	m.height = 24

	// Select all with 'A'
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'A'}))
	if len(m.selected) != 3 {
		t.Errorf("expected 3 selected, got %d", len(m.selected))
	}

	// Deselect all with 'A' again
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'A'}))
	if len(m.selected) != 0 {
		t.Errorf("expected 0 selected, got %d", len(m.selected))
	}
}

func TestCursorMovement(t *testing.T) {
	m := New(nil, 0)
	m.nodes = makeTestNodes()
	m.applyFilter()
	m.width = 80
	m.height = 24

	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.cursor)
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0 after up, got %d", m.cursor)
	}

	// Don't go negative
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.cursor)
	}
}

func TestFilter(t *testing.T) {
	m := New(nil, 0)
	m.nodes = makeTestNodes()
	m.applyFilter()
	m.width = 80
	m.height = 24

	// Enter filter mode
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '/'}))
	if !m.filterActive {
		t.Error("expected filter to be active")
	}

	// Type "worker"
	for _, c := range "worker" {
		m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: rune(c)}))
	}

	// Apply filter
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.filterActive {
		t.Error("expected filter to be inactive after enter")
	}
	if len(m.filtered) != 1 {
		t.Errorf("expected 1 filtered node, got %d", len(m.filtered))
	}
}

func TestCtrlXInDetailViewEmitsReset(t *testing.T) {
	m := New(nil, 0)
	m.nodes = makeTestNodes()
	m.applyFilter()
	m.detailView = true
	m.cursor = 0

	_, cmd := m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("expected a command after ctrl+x in detail view")
	}
	msg := cmd()
	req, ok := msg.(shared.NodeResetRequestMsg)
	if !ok {
		t.Fatalf("expected NodeResetRequestMsg, got %T", msg)
	}
	if req.Node != "cp-1" {
		t.Errorf("expected node cp-1, got %q", req.Node)
	}
}

func TestTickMsg_TriggersRefresh(t *testing.T) {
	m := New(nil, 0)
	_, cmd := m.Update(shared.TickMsg{})
	if cmd == nil {
		t.Error("expected TickMsg to produce a fetch command")
	}
}

func TestSelectedNodes_CursorFallback(t *testing.T) {
	m := New(nil, 0)
	m.nodes = makeTestNodes()
	m.applyFilter()

	nodes := m.SelectedNodes()
	if len(nodes) != 1 || nodes[0].Hostname != "cp-1" {
		t.Errorf("expected cursor node cp-1 as fallback, got %v", nodes)
	}
}
