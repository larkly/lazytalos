package servicelist

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestBuildRows_SortedByNodeThenService(t *testing.T) {
	byNode := map[string][]ServiceListRow{
		"node-b": {
			{Node: "node-b", ServiceID: "kubelet", State: "Running"},
			{Node: "node-b", ServiceID: "apid", State: "Running"},
		},
		"node-a": {
			{Node: "node-a", ServiceID: "etcd", State: "Running"},
			{Node: "node-a", ServiceID: "apid", State: "Running"},
		},
	}

	rows := BuildRows(byNode)
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}

	// node-a first, then node-b
	if rows[0].Node != "node-a" || rows[0].ServiceID != "apid" {
		t.Errorf("row[0] = %s/%s, want node-a/apid", rows[0].Node, rows[0].ServiceID)
	}
	if rows[1].Node != "node-a" || rows[1].ServiceID != "etcd" {
		t.Errorf("row[1] = %s/%s, want node-a/etcd", rows[1].Node, rows[1].ServiceID)
	}
	if rows[2].Node != "node-b" || rows[2].ServiceID != "apid" {
		t.Errorf("row[2] = %s/%s, want node-b/apid", rows[2].Node, rows[2].ServiceID)
	}
}

func TestGroupByNodeToggle(t *testing.T) {
	m := New(nil, 0)
	if m.groupByNode {
		t.Error("expected groupByNode to be false initially")
	}

	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'g'}))
	if !m.groupByNode {
		t.Error("expected groupByNode to be true after 'g'")
	}

	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'g'}))
	if m.groupByNode {
		t.Error("expected groupByNode to be false after second 'g'")
	}
}

func TestTickMsg_TriggersRefresh(t *testing.T) {
	m := New(nil, 0)
	_, cmd := m.Update(shared.TickMsg{})
	if cmd == nil {
		t.Error("expected TickMsg to produce a fetch command")
	}
}
