package network

import (
	"testing"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/resources"
)

func TestNew(t *testing.T) {
	m := New(nil, 30*time.Second)
	if m.client != nil {
		t.Error("expected nil client")
	}
	if !m.loading {
		t.Error("expected loading=true on init")
	}
	if m.subTab != 0 {
		t.Errorf("expected subTab=0, got %d", m.subTab)
	}
	if m.refreshInterval != 30*time.Second {
		t.Errorf("expected refreshInterval=30s, got %v", m.refreshInterval)
	}
}

func TestSetSize(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestSubTabCycle(t *testing.T) {
	m := New(nil, 30*time.Second)

	// ] cycles forward: 0 -> 1 -> 2 -> 0
	nextKey := tea.KeyMsg(tea.KeyPressMsg{Code: ']'})
	for _, expected := range []int{1, 2, 0} {
		var cmd tea.Cmd
		m, cmd = m.Update(nextKey)
		_ = cmd
		if m.subTab != expected {
			t.Errorf("expected subTab=%d after ], got %d", expected, m.subTab)
		}
	}

	// [ cycles backward: 0 -> 2 -> 1 -> 0
	prevKey := tea.KeyMsg(tea.KeyPressMsg{Code: '['})
	for _, expected := range []int{2, 1, 0} {
		var cmd tea.Cmd
		m, cmd = m.Update(prevKey)
		_ = cmd
		if m.subTab != expected {
			t.Errorf("expected subTab=%d after [, got %d", expected, m.subTab)
		}
	}
}

func TestSubTabCycleResetsCursor(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.cursor = 5

	nextKey := tea.KeyMsg(tea.KeyPressMsg{Code: ']'})
	m, _ = m.Update(nextKey)
	if m.cursor != 0 {
		t.Errorf("expected cursor reset to 0 on sub-tab change, got %d", m.cursor)
	}
}

func TestHints(t *testing.T) {
	m := New(nil, 30*time.Second)
	hints := m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints")
	}
}

func TestViewEmptyBeforeLoad(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.SetSize(120, 40)
	// loading=true, no data — should show loading message
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view even during loading")
	}
}

func TestViewWithData(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.SetSize(120, 40)
	m.loading = false

	// Inject loaded message
	msg := networkLoadedMsg{
		addresses: []resources.AddressStatus{
			{NodeHostname: "node-1", Interface: "eth0", Address: "10.0.0.1/24", Scope: "global", Flags: "permanent"},
		},
		routes: []resources.RouteStatus{
			{NodeHostname: "node-1", Destination: "0.0.0.0/0", Gateway: "10.0.0.1", Interface: "eth0", Metric: 100},
		},
		dnsUpstreams: []resources.DNSUpstream{
			{NodeHostname: "node-1", Address: "8.8.8.8"},
		},
	}
	m, _ = m.Update(msg)

	// Check addresses sub-tab
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view with data")
	}

	// Check routes sub-tab
	m.subTab = subTabRoutes
	v = m.View()
	if v == "" {
		t.Error("expected non-empty view for routes sub-tab")
	}

	// Check DNS sub-tab
	m.subTab = subTabDNS
	v = m.View()
	if v == "" {
		t.Error("expected non-empty view for DNS sub-tab")
	}
}

func TestLeftRightNotConsumed(t *testing.T) {
	// Left/Right keys should be passed through (not alter subTab or cursor).
	m := New(nil, 30*time.Second)
	initial := m.subTab

	leftKey := tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyLeft})
	m, _ = m.Update(leftKey)
	if m.subTab != initial {
		t.Errorf("left key should not change subTab; got %d", m.subTab)
	}

	rightKey := tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyRight})
	m, _ = m.Update(rightKey)
	if m.subTab != initial {
		t.Errorf("right key should not change subTab; got %d", m.subTab)
	}
}
