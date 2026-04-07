package logviewer

import (
	"testing"
	"time"

	"charm.land/bubbletea/v2"
)

func TestRingBuffer_CapsAtMaxLines(t *testing.T) {
	m := New(nil, 0)
	m.maxLines = 10

	for i := 0; i < 20; i++ {
		m.AppendLine(LogLine{
			Node:    "node-1",
			Service: "kubelet",
			Text:    "line",
			T:       time.Now(),
		})
	}

	if len(m.Lines()) != 10 {
		t.Errorf("expected 10 lines, got %d", len(m.Lines()))
	}
}

func TestRingBuffer_DropsOldest(t *testing.T) {
	m := New(nil, 0)
	m.maxLines = 3

	m.AppendLine(LogLine{Text: "first"})
	m.AppendLine(LogLine{Text: "second"})
	m.AppendLine(LogLine{Text: "third"})
	m.AppendLine(LogLine{Text: "fourth"})

	lines := m.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0].Text != "second" {
		t.Errorf("expected oldest to be 'second', got %q", lines[0].Text)
	}
	if lines[2].Text != "fourth" {
		t.Errorf("expected newest to be 'fourth', got %q", lines[2].Text)
	}
}

func TestStreamKey_Identity(t *testing.T) {
	k1 := StreamKey{Node: "a", Service: "b"}
	k2 := StreamKey{Node: "a", Service: "b"}
	k3 := StreamKey{Node: "a", Service: "c"}

	if k1 != k2 {
		t.Error("identical StreamKeys should be equal")
	}
	if k1 == k3 {
		t.Error("different StreamKeys should not be equal")
	}
}

func TestSetNodes(t *testing.T) {
	m := New(nil, 0)
	m.SetNodes([]string{"node-1", "node-2"})
	if len(m.nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(m.nodes))
	}
}

func TestDefaultServices(t *testing.T) {
	m := New(nil, 0)
	if len(m.services) != len(defaultServices) {
		t.Errorf("expected %d services, got %d", len(defaultServices), len(m.services))
	}
}

func TestFollowToggle(t *testing.T) {
	m := New(nil, 0)
	if !m.follow {
		t.Error("expected follow to be true by default")
	}
	// Toggle via F key
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'F'}))
	if m.follow {
		t.Error("expected follow to be false after F")
	}
}
