package storageview

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestNew(t *testing.T) {
	m := New(nil, 0)

	if m.activeTab != subTabDevices {
		t.Errorf("expected activeTab = subTabDevices, got %d", m.activeTab)
	}
	if !m.loading {
		t.Error("expected loading = true")
	}
	if !m.focusLeft {
		t.Error("expected focusLeft = true")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor = 0, got %d", m.cursor)
	}
	if m.filter != "" {
		t.Errorf("expected empty filter, got %q", m.filter)
	}
	if m.filterActive {
		t.Error("expected filterActive = false")
	}
	if len(m.devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(m.devices))
	}
	if len(m.volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(m.volumes))
	}
}

func TestSubTabSwitch(t *testing.T) {
	m := New(nil, 0)
	m.loading = false

	// Tab cycles forward.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.activeTab != subTabVolumes {
		t.Errorf("expected subTabVolumes after Tab, got %d", m.activeTab)
	}

	// Tab wraps around.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.activeTab != subTabDevices {
		t.Errorf("expected subTabDevices after second Tab, got %d", m.activeTab)
	}

	// ShiftTab cycles backward.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}))
	if m.activeTab != subTabVolumes {
		t.Errorf("expected subTabVolumes after ShiftTab, got %d", m.activeTab)
	}

	// ShiftTab from Devices wraps to Volumes.
	m.activeTab = subTabDevices
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}))
	if m.activeTab != subTabVolumes {
		t.Errorf("expected subTabVolumes after ShiftTab from Devices, got %d", m.activeTab)
	}
}

func TestCursorNavigation(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.SetSize(120, 30)
	m.devices = []deviceRow{
		{Node: "node-1", DevPath: "/dev/sda", PrettySize: "50 GiB", Model: "VBOX"},
		{Node: "node-1", DevPath: "/dev/sdb", PrettySize: "100 GiB", Model: "NVME"},
		{Node: "node-2", DevPath: "/dev/sda", PrettySize: "50 GiB", Model: "VBOX"},
	}

	// Down moves cursor.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'j'}))
	if m.cursor != 1 {
		t.Errorf("expected cursor = 1 after down, got %d", m.cursor)
	}

	// Down again.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'j'}))
	if m.cursor != 2 {
		t.Errorf("expected cursor = 2 after second down, got %d", m.cursor)
	}

	// Down at end stays at end.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'j'}))
	if m.cursor != 2 {
		t.Errorf("expected cursor = 2 (clamped), got %d", m.cursor)
	}

	// Up moves cursor back.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'k'}))
	if m.cursor != 1 {
		t.Errorf("expected cursor = 1 after up, got %d", m.cursor)
	}

	// Up at top stays at top.
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'k'}))
	if m.cursor != 0 {
		t.Errorf("expected cursor = 0 (clamped), got %d", m.cursor)
	}
}

func TestFilter(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.SetSize(120, 30)
	m.devices = []deviceRow{
		{Node: "cp-1", DevPath: "/dev/sda", PrettySize: "50 GiB", Model: "VBOX"},
		{Node: "worker-1", DevPath: "/dev/sda", PrettySize: "100 GiB", Model: "NVME"},
		{Node: "cp-2", DevPath: "/dev/sdb", PrettySize: "25 GiB", Model: "SAMSUNG"},
	}

	// Enter filter mode.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '/'}))
	if !m.filterActive {
		t.Error("expected filterActive = true after '/'")
	}

	// Type filter text.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'c'}))
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: 'p'}))
	if m.filter != "cp" {
		t.Errorf("expected filter = %q, got %q", "cp", m.filter)
	}

	// Check filtered devices.
	filtered := m.filteredDevices()
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered devices, got %d", len(filtered))
	}

	// Confirm filter with Enter.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.filterActive {
		t.Error("expected filterActive = false after Enter")
	}
	if m.filter != "cp" {
		t.Errorf("expected filter to remain %q after Enter, got %q", "cp", m.filter)
	}

	// Backspace in filter mode.
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '/'}))
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyBackspace}))
	if m.filter != "c" {
		t.Errorf("expected filter = %q after backspace, got %q", "c", m.filter)
	}
}

func TestTickMsg_TriggersRefresh(t *testing.T) {
	m := New(nil, 0)
	_, cmd := m.Update(shared.TickMsg{})
	if cmd == nil {
		t.Error("expected TickMsg to produce a fetch command")
	}
}

func TestSetSize(t *testing.T) {
	m := New(nil, 0)
	m.SetSize(100, 50)
	if m.width != 100 {
		t.Errorf("expected width = 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height = 50, got %d", m.height)
	}
}

func TestHints(t *testing.T) {
	m := New(nil, 0)

	hints := m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints")
	}

	m.filterActive = true
	hints = m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints in filter mode")
	}
	if hints == m.Hints() {
		// This is fine, just verify it returns something.
	}
}

func TestSubTabSwitchResetsCursor(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.cursor = 5

	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	if m.cursor != 0 {
		t.Errorf("expected cursor reset to 0 on tab switch, got %d", m.cursor)
	}
}

func TestFilterVolumes(t *testing.T) {
	m := New(nil, 0)
	m.loading = false
	m.activeTab = subTabVolumes
	m.volumes = []volumeRow{
		{Node: "cp-1", ID: "EPHEMERAL", Phase: "ready", PrettySize: "10 GiB", Location: "/dev/sda3"},
		{Node: "cp-1", ID: "STATE", Phase: "ready", PrettySize: "100 MiB", Location: "/dev/sda2"},
		{Node: "worker-1", ID: "EPHEMERAL", Phase: "ready", PrettySize: "10 GiB", Location: "/dev/sda3"},
	}

	m.filter = "STATE"
	filtered := m.filteredVolumes()
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered volume, got %d", len(filtered))
	}
}

func TestViewEmptyDimensions(t *testing.T) {
	m := New(nil, 0)
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got %q", view)
	}
}
