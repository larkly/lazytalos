package containers

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

func makeTestContainers() []containerRow {
	return []containerRow{
		{Node: "cp-1", Namespace: "system", Name: "machined", ID: "abc123", Image: "ghcr.io/siderolabs/machined:v1.12", Status: "RUNNING", PID: 100, PodID: ""},
		{Node: "cp-1", Namespace: "k8s.io", Name: "coredns", ID: "def456", Image: "registry.k8s.io/coredns:v1.11", Status: "RUNNING", PID: 200, PodID: "pod-1"},
		{Node: "worker-1", Namespace: "k8s.io", Name: "kube-proxy", ID: "ghi789", Image: "registry.k8s.io/kube-proxy:v1.31", Status: "STOPPED", PID: 0, PodID: "pod-2"},
	}
}

func TestNew(t *testing.T) {
	m := New(nil, 0)
	if m.namespace != "all" {
		t.Errorf("expected namespace 'all', got %q", m.namespace)
	}
	if !m.loading {
		t.Error("expected loading to be true")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
	if m.filterActive {
		t.Error("expected filterActive to be false")
	}
	if m.detailView {
		t.Error("expected detailView to be false")
	}
}

func TestNamespaceToggle(t *testing.T) {
	m := New(nil, 0)
	m.containers = makeTestContainers()
	m.applyFilter()
	m.loading = false

	if m.namespace != "all" {
		t.Fatalf("expected initial namespace 'all', got %q", m.namespace)
	}

	// Cycle: all -> k8s.io
	m.cycleNamespace()
	if m.namespace != "k8s.io" {
		t.Errorf("expected 'k8s.io', got %q", m.namespace)
	}

	// Cycle: k8s.io -> system
	m.cycleNamespace()
	if m.namespace != "system" {
		t.Errorf("expected 'system', got %q", m.namespace)
	}

	// Cycle: system -> all
	m.cycleNamespace()
	if m.namespace != "all" {
		t.Errorf("expected 'all', got %q", m.namespace)
	}
}

func TestFilter(t *testing.T) {
	m := New(nil, 0)
	m.containers = makeTestContainers()
	m.applyFilter()
	m.loading = false
	m.width = 120
	m.height = 24

	// Enter filter mode
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: '/'}))
	if !m.filterActive {
		t.Error("expected filter to be active")
	}

	// Type "coredns"
	for _, c := range "coredns" {
		m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: rune(c)}))
	}

	// Apply filter
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.filterActive {
		t.Error("expected filter to be inactive after enter")
	}
	if len(m.filtered) != 1 {
		t.Errorf("expected 1 filtered container, got %d", len(m.filtered))
	}
	if m.filtered[0].Name != "coredns" {
		t.Errorf("expected filtered container 'coredns', got %q", m.filtered[0].Name)
	}
}

func TestCursorNavigation(t *testing.T) {
	m := New(nil, 0)
	m.containers = makeTestContainers()
	m.applyFilter()
	m.loading = false
	m.width = 120
	m.height = 24

	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.cursor)
	}

	// Move down again
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2 after second down, got %d", m.cursor)
	}

	// Don't go past end
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", m.cursor)
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after up, got %d", m.cursor)
	}

	// Move up twice
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyUp}))
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.cursor)
	}
}

func TestDetailView(t *testing.T) {
	m := New(nil, 0)
	m.containers = makeTestContainers()
	m.applyFilter()
	m.loading = false
	m.width = 120
	m.height = 24

	// Enter detail view
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if !m.detailView {
		t.Error("expected detailView to be true after Enter")
	}

	// Verify detail view renders without panic
	view := m.View()
	if !contains(view, "machined") {
		t.Error("expected detail view to contain container name 'machined'")
	}

	// Exit detail view with Esc
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.detailView {
		t.Error("expected detailView to be false after Esc")
	}
}

func TestTickMsg(t *testing.T) {
	m := New(nil, 0)
	_, cmd := m.Update(shared.TickMsg{})
	if cmd == nil {
		t.Error("expected TickMsg to produce a fetch command")
	}
}

func TestViewEmptySize(t *testing.T) {
	m := New(nil, 0)
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view with zero size, got %q", view)
	}
}

func TestViewLoading(t *testing.T) {
	m := New(nil, 0)
	m.width = 120
	m.height = 24
	view := m.View()
	if !contains(view, "Loading containers") {
		t.Error("expected loading message")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
