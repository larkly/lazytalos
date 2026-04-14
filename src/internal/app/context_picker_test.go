package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Regression: pressing C after a single-context auto-connect must show the
// picker, not the leftover "Connecting..." splash from autoSelect.
func TestContextPicker_ShowsPickerAfterAutoConnect(t *testing.T) {
	m := New(Options{
		Talosconfig:     "/nonexistent/talosconfig",
		RefreshInterval: 5 * time.Second,
	})
	m.autoSelect = "tnn3-demo"
	m.view = viewDashboard

	r, _ := m.Update(tea.KeyPressMsg{Text: "C", Code: 'C'})
	after := r.(Model)

	if after.view != viewContextPicker {
		t.Fatalf("view = %d, want viewContextPicker (%d)", after.view, viewContextPicker)
	}
	if after.autoSelect != "" {
		t.Errorf("autoSelect = %q, want empty", after.autoSelect)
	}
	if strings.Contains(after.viewContent(), "Connecting to") {
		t.Errorf("render still shows 'Connecting to' splash after pressing C")
	}
}
