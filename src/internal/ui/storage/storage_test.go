package storage

import (
	"testing"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/resources"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestNew(t *testing.T) {
	m := New(nil, 30*time.Second)
	if m.client != nil {
		t.Error("expected nil client")
	}
	if !m.loading {
		t.Error("expected loading=true after New")
	}
	if m.subTab != subTabDevices {
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
		t.Errorf("SetSize: got %dx%d, want 120x40", m.width, m.height)
	}
}

func TestSubTabCycle(t *testing.T) {
	m := New(nil, 30*time.Second)

	nextKey := tea.KeyMsg(tea.KeyPressMsg{Code: ']'})
	prevKey := tea.KeyMsg(tea.KeyPressMsg{Code: '['})

	// ] cycles forward: 0 -> 1 -> 0
	for _, expected := range []int{subTabVolumes, subTabDevices} {
		var cmd tea.Cmd
		m, cmd = m.Update(nextKey)
		_ = cmd
		if m.subTab != expected {
			t.Errorf("expected subTab=%d after ], got %d", expected, m.subTab)
		}
	}

	// [ cycles backward: 0 -> 1 -> 0
	for _, expected := range []int{subTabVolumes, subTabDevices} {
		var cmd tea.Cmd
		m, cmd = m.Update(prevKey)
		_ = cmd
		if m.subTab != expected {
			t.Errorf("expected subTab=%d after [, got %d", expected, m.subTab)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input    uint64
		contains string
	}{
		{0, "0"},
		{512 * 1024 * 1024, "512.0 MiB"},
		{1073741824, "1.0 GiB"},
		{137438953472, "128.0 GiB"}, // 128 GiB
		{536870912, "512.0 MiB"},    // 512 MiB (< 1 GiB)
	}

	for _, tc := range tests {
		got := formatSize(tc.input)
		if got != tc.contains {
			t.Errorf("formatSize(%d) = %q, want %q", tc.input, got, tc.contains)
		}
	}
}

func TestStorageLoadedMsg(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = true

	msg := storageLoadedMsg{
		devices: []resources.BlockDevice{
			{NodeHostname: "node1", Name: "sda", DevType: "disk", Size: 137438953472},
		},
		vols: []resources.DiscoveredVolume{
			{NodeHostname: "node1", Name: "sda1", FSType: "vfat", Label: "EFI", UUID: "ABCD-1234", Size: 536870912},
		},
		statuses: []resources.VolumeStatus{
			{NodeHostname: "node1", Name: "EPHEMERAL", Phase: "READY", MountSpec: "/var"},
		},
	}

	m2, _ := m.Update(msg)
	if m2.loading {
		t.Error("expected loading=false after storageLoadedMsg")
	}
	if len(m2.devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(m2.devices))
	}
	if len(m2.discoveredVols) != 1 {
		t.Errorf("expected 1 discovered vol, got %d", len(m2.discoveredVols))
	}
	if len(m2.volStatuses) != 1 {
		t.Errorf("expected 1 vol status, got %d", len(m2.volStatuses))
	}
}

func TestViewRendersSubTabBar(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = false
	m.SetSize(120, 30)

	view := m.View()
	if !containsAny(view, "Devices", "Volumes") {
		t.Error("View() should contain sub-tab names")
	}
}

func TestLeftRightNotConsumed(t *testing.T) {
	m := New(nil, 30*time.Second)
	initialSubTab := m.subTab

	leftKey := tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyLeft})
	m2, _ := m.Update(leftKey)
	if m2.subTab != initialSubTab {
		t.Errorf("left key should not change subTab; got %d", m2.subTab)
	}

	rightKey := tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyRight})
	m3, _ := m.Update(rightKey)
	if m3.subTab != initialSubTab {
		t.Errorf("right key should not change subTab; got %d", m3.subTab)
	}
}

func TestTickMsgTriggersRefresh(t *testing.T) {
	m := New(nil, 30*time.Second)
	_, cmd := m.Update(shared.TickMsg{})
	if cmd == nil {
		t.Error("TickMsg should return a non-nil command")
	}
}

func TestHints(t *testing.T) {
	m := New(nil, 30*time.Second)
	hints := m.Hints()
	if hints == "" {
		t.Error("Hints() should not be empty")
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) == 0 {
			continue
		}
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
