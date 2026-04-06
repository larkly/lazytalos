package app

import (
	"testing"
	"time"
)

func TestNew_DefaultOptions(t *testing.T) {
	m := New(Options{
		Talosconfig:     "/nonexistent/talosconfig",
		RefreshInterval: 10 * time.Second,
		Version:         "v0.1.0",
	})

	if m.version != "v0.1.0" {
		t.Errorf("version = %q, want %q", m.version, "v0.1.0")
	}
	if m.refreshInterval != 10*time.Second {
		t.Errorf("refreshInterval = %v, want %v", m.refreshInterval, 10*time.Second)
	}
	if m.view != viewContextPicker {
		t.Errorf("view = %d, want %d (viewContextPicker)", m.view, viewContextPicker)
	}
	if len(m.tabs) != 4 {
		t.Errorf("tabs len = %d, want 4", len(m.tabs))
	}
	if len(m.tabInited) != 4 {
		t.Errorf("tabInited len = %d, want 4", len(m.tabInited))
	}
}

func TestNew_DefaultRefreshInterval(t *testing.T) {
	m := New(Options{
		Talosconfig: "/nonexistent/talosconfig",
	})

	if m.refreshInterval != 5*time.Second {
		t.Errorf("refreshInterval = %v, want %v", m.refreshInterval, 5*time.Second)
	}
}

func TestSwitchTab(t *testing.T) {
	m := New(Options{
		Talosconfig:     "/nonexistent/talosconfig",
		RefreshInterval: 5 * time.Second,
	})
	// Need to be in a top-level view for switchTab to work;
	// simulate post-connect state by setting view
	m.view = viewDashboard
	m.tabInited[0] = true

	m, _ = m.switchTab(1)
	if m.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1", m.activeTab)
	}
	if m.view != viewNodeList {
		t.Errorf("view = %d, want %d (viewNodeList)", m.view, viewNodeList)
	}
}

func TestSwitchTab_OutOfBounds(t *testing.T) {
	m := New(Options{
		Talosconfig: "/nonexistent/talosconfig",
	})

	m2, cmd := m.switchTab(-1)
	if cmd != nil {
		t.Error("expected nil cmd for out-of-bounds tab")
	}
	if m2.activeTab != m.activeTab {
		t.Error("activeTab should not change for out-of-bounds")
	}

	m2, cmd = m.switchTab(99)
	if cmd != nil {
		t.Error("expected nil cmd for out-of-bounds tab")
	}
}

func TestShouldRestart_Default(t *testing.T) {
	m := New(Options{
		Talosconfig: "/nonexistent/talosconfig",
	})

	if m.ShouldRestart() {
		t.Error("ShouldRestart() = true, want false")
	}
}

func TestShouldRestart_WhenSet(t *testing.T) {
	m := New(Options{
		Talosconfig: "/nonexistent/talosconfig",
	})
	m.restart = true

	if !m.ShouldRestart() {
		t.Error("ShouldRestart() = false, want true")
	}
}

func TestIsTopLevelView(t *testing.T) {
	m := New(Options{
		Talosconfig: "/nonexistent/talosconfig",
	})

	m.view = viewContextPicker
	if m.isTopLevelView() {
		t.Error("context picker should not be a top-level view")
	}

	m.view = viewDashboard
	if !m.isTopLevelView() {
		t.Error("dashboard should be a top-level view")
	}

	m.view = viewNodeList
	if !m.isTopLevelView() {
		t.Error("node list should be a top-level view")
	}

	m.view = viewServiceList
	if !m.isTopLevelView() {
		t.Error("service list should be a top-level view")
	}

	m.view = viewLogViewer
	if !m.isTopLevelView() {
		t.Error("log viewer should be a top-level view")
	}
}
