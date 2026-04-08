package help

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New()
	if m.IsVisible() {
		t.Error("New() model should not be visible")
	}
}

func TestOpenCycles(t *testing.T) {
	m := New()

	// First ? → profiled (visible)
	m.Open("nodes")
	if !m.IsVisible() {
		t.Error("after first Open() model should be visible")
	}
	if m.tier != tierProfiled {
		t.Errorf("tier = %d, want tierProfiled(%d)", m.tier, tierProfiled)
	}

	// Second ? → full (visible)
	m.Open("nodes")
	if !m.IsVisible() {
		t.Error("after second Open() model should be visible")
	}
	if m.tier != tierFull {
		t.Errorf("tier = %d, want tierFull(%d)", m.tier, tierFull)
	}

	// Third ? → closed
	m.Open("nodes")
	if m.IsVisible() {
		t.Error("after third Open() model should be hidden")
	}
}

func TestProfiledShowsViewSection(t *testing.T) {
	m := New()
	m.SetSize(120, 50)
	m.Open("nodes")

	// Should contain GLOBAL and NODES but not SERVICES
	hasGlobal := false
	hasNodes := false
	hasServices := false
	for _, line := range m.lines {
		if contains(line, "GLOBAL") {
			hasGlobal = true
		}
		if contains(line, "NODES") {
			hasNodes = true
		}
		if contains(line, "SERVICES") {
			hasServices = true
		}
	}
	if !hasGlobal {
		t.Error("profiled mode should contain GLOBAL section")
	}
	if !hasNodes {
		t.Error("profiled mode for 'nodes' should contain NODES section")
	}
	if hasServices {
		t.Error("profiled mode for 'nodes' should not contain SERVICES section")
	}
}

func TestFullShowsAllSections(t *testing.T) {
	m := New()
	m.SetSize(120, 50)
	m.Open("nodes") // profiled
	m.Open("nodes") // full

	hasServices := false
	hasContainers := false
	for _, line := range m.lines {
		if contains(line, "SERVICES") {
			hasServices = true
		}
		if contains(line, "CONTAINERS") {
			hasContainers = true
		}
	}
	if !hasServices {
		t.Error("full mode should contain SERVICES section")
	}
	if !hasContainers {
		t.Error("full mode should contain CONTAINERS section")
	}
}

func TestAnyKeyCloses(t *testing.T) {
	m := New()
	m.Open("nodes")
	if !m.IsVisible() {
		t.Fatal("precondition: model should be visible")
	}

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.IsVisible() {
		t.Error("after esc key, overlay should close")
	}
}

func TestUpDownScrolls(t *testing.T) {
	m := New()
	m.SetSize(120, 50)
	m.Open("nodes")

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if !m.IsVisible() {
		t.Error("Down key should not close the overlay")
	}

	m.scrollY = 0
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if !m.IsVisible() {
		t.Error("Up key should not close the overlay")
	}
	if m.scrollY < 0 {
		t.Errorf("scrollY = %d, should not be negative", m.scrollY)
	}

	m.scrollY = 3
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.scrollY != 2 {
		t.Errorf("scrollY = %d, want 2", m.scrollY)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
