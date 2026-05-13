package settings

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/config"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestNew(t *testing.T) {
	cfg := config.Defaults()
	m := New(&cfg)
	if m.Visible {
		t.Error("expected not visible initially")
	}
	if m.totalItems() == 0 {
		t.Error("expected items")
	}
}

func TestOpenClose(t *testing.T) {
	cfg := config.Defaults()
	m := New(&cfg)
	m.Width = 80
	m.Height = 40

	m.Open()
	if !m.Visible {
		t.Error("expected visible after Open")
	}

	// Esc should close
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.Visible {
		t.Error("expected not visible after Esc")
	}
}

func TestBrowseDown(t *testing.T) {
	cfg := config.Defaults()
	m := New(&cfg)
	m.Open()

	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}

	// Press down
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}
}

func TestToggleBool(t *testing.T) {
	cfg := config.Defaults()
	m := New(&cfg)
	m.Width = 80
	m.Height = 40
	m.Open()

	// Move to "Plain mode" (index 1)
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 (plain mode), got %d", m.cursor)
	}

	before := cfg.General.PlainMode
	// Press enter to toggle
	m, _ = m.Update(tea.KeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if cfg.General.PlainMode == before {
		t.Error("expected PlainMode to toggle")
	}
}

func TestRender(t *testing.T) {
	cfg := config.Defaults()
	m := New(&cfg)
	m.Width = 80
	m.Height = 40
	m.Open()

	out := m.Render()
	if out == "" {
		t.Error("expected non-empty render")
	}
	if !contains(out, "Settings") {
		t.Error("expected 'Settings' title in render")
	}
	if !contains(out, "General") {
		t.Error("expected 'General' category in render")
	}
	if !contains(out, "Thresholds") {
		t.Error("expected 'Thresholds' category in render")
	}
}

func contains(s, sub string) bool {
	// Strip ANSI for checking
	_ = shared.IsPlainMode // ensure shared is used
	return len(s) > 0 && len(sub) > 0 && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
