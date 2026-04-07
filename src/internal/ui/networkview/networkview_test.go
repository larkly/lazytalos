package networkview

import (
	"testing"
	"time"

	"charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New(nil, 5*time.Second)

	if m.activeTab != subTabAddresses {
		t.Errorf("expected initial tab to be Addresses (0), got %d", m.activeTab)
	}
	if !m.focusLeft {
		t.Error("expected focusLeft to be true initially")
	}
	if !m.loading {
		t.Error("expected loading to be true initially")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.cursor)
	}
	if m.filterActive {
		t.Error("expected filterActive to be false initially")
	}
	if m.refreshInterval != 5*time.Second {
		t.Errorf("expected refreshInterval 5s, got %v", m.refreshInterval)
	}
}

func TestSubTabSwitch(t *testing.T) {
	m := New(nil, 0)
	m.loading = false

	// Tab cycles forward: Addresses -> Routes
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.activeTab != subTabRoutes {
		t.Errorf("expected Routes (1) after Tab, got %d", m.activeTab)
	}

	// Tab again: Routes -> DNS
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.activeTab != subTabDNS {
		t.Errorf("expected DNS (2) after Tab, got %d", m.activeTab)
	}

	// Tab wraps: DNS -> Addresses
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.activeTab != subTabAddresses {
		t.Errorf("expected Addresses (0) after Tab wrap, got %d", m.activeTab)
	}

	// ShiftTab goes backward: Addresses -> DNS
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}))
	if m.activeTab != subTabDNS {
		t.Errorf("expected DNS (2) after ShiftTab, got %d", m.activeTab)
	}
}

func TestCursorNavigation(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.focusLeft = false
	m.height = 20
	m.width = 80

	// Add some address data
	m.addresses = []addressRow{
		{Node: "node-1", LinkName: "eth0", Address: "10.0.0.1/24", Family: "IPv4", Scope: "global"},
		{Node: "node-1", LinkName: "lo", Address: "127.0.0.1/8", Family: "IPv4", Scope: "host"},
		{Node: "node-2", LinkName: "eth0", Address: "10.0.0.2/24", Family: "IPv4", Scope: "global"},
	}

	// Down moves cursor
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after Down, got %d", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 after Down, got %d", m.cursor)
	}

	// Down at bottom stays at bottom
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 (clamped), got %d", m.cursor)
	}

	// Up moves cursor back
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after Up, got %d", m.cursor)
	}

	// Up at top stays at top
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 (clamped), got %d", m.cursor)
	}
}

func TestFilter(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.focusLeft = false
	m.height = 20
	m.width = 80

	m.addresses = []addressRow{
		{Node: "node-1", LinkName: "eth0", Address: "10.0.0.1/24", Family: "IPv4", Scope: "global"},
		{Node: "node-1", LinkName: "lo", Address: "127.0.0.1/8", Family: "IPv4", Scope: "host"},
		{Node: "node-2", LinkName: "eth0", Address: "10.0.0.2/24", Family: "IPv4", Scope: "global"},
	}

	// Activate filter with /
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '/'}))
	if !m.filterActive {
		t.Error("expected filterActive after /")
	}

	// Type filter text "127" to match only the loopback address
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '1', Text: "1"}))
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '2', Text: "2"}))
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '7', Text: "7"}))
	if m.filter != "127" {
		t.Errorf("expected filter '127', got %q", m.filter)
	}

	// Filtered rows should only show the 127.0.0.1 address row
	filtered := m.filteredAddresses()
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered row, got %d", len(filtered))
	}
	if len(filtered) > 0 && filtered[0].Address != "127.0.0.1/8" {
		t.Errorf("expected filtered row address '127.0.0.1/8', got %q", filtered[0].Address)
	}

	// Esc cancels filter
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.filterActive {
		t.Error("expected filterActive to be false after Esc")
	}
	if m.filter != "" {
		t.Errorf("expected empty filter after Esc, got %q", m.filter)
	}
}

func TestSetSize(t *testing.T) {
	m := New(nil, 0)
	m.SetSize(120, 40)
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestViewEmpty(t *testing.T) {
	m := New(nil, 0)
	// Zero size returns empty
	v := m.View()
	if v != "" {
		t.Errorf("expected empty view with zero size, got %q", v)
	}
}

func TestSubTabSwitchResetsState(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.cursor = 5
	m.scrollOff = 3
	m.filter = "test"

	// Switch tab
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.cursor != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.cursor)
	}
	if m.scrollOff != 0 {
		t.Errorf("expected scrollOff reset to 0, got %d", m.scrollOff)
	}
	if m.filter != "" {
		t.Errorf("expected filter reset to empty, got %q", m.filter)
	}
}

func TestFocusSwitch(t *testing.T) {
	m := New(nil, 0)
	m.loading = false

	// Start on left pane
	if !m.focusLeft {
		t.Error("expected focusLeft initially")
	}

	// Enter switches to right pane
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.focusLeft {
		t.Error("expected focusLeft false after Enter")
	}

	// Esc goes back to left pane
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if !m.focusLeft {
		t.Error("expected focusLeft true after Esc")
	}
}

func TestHints(t *testing.T) {
	m := New(nil, 0)
	m.loading = false

	// Left pane hints
	hints := m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints for left pane")
	}

	// Right pane hints
	m.focusLeft = false
	hints = m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints for right pane")
	}

	// Filter hints
	m.filterActive = true
	hints = m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints for filter mode")
	}
}
