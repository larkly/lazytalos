package configeditor

import (
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

func testModel() Model {
	m := Model{
		node:   "test-node",
		lines:  []string{"machine:", "  type: controlplane", "  network:", "    hostname: test", ""},
		width:  80,
		height: 24,
	}
	return m
}

func TestNew(t *testing.T) {
	m := New(nil, "node1")
	if !m.loading {
		t.Error("expected loading=true")
	}
	if m.node != "node1" {
		t.Errorf("node = %q, want %q", m.node, "node1")
	}
}

func TestCursorMovement(t *testing.T) {
	m := testModel()

	// Down
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", m.cursorRow)
	}

	// Right
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.cursorCol != 1 {
		t.Errorf("cursorCol = %d, want 1", m.cursorCol)
	}

	// Left
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", m.cursorCol)
	}

	// Up
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursorRow != 0 {
		t.Errorf("cursorRow = %d, want 0", m.cursorRow)
	}
}

func TestTextInsert(t *testing.T) {
	m := testModel()
	m.cursorRow = 0
	m.cursorCol = 0

	// Type 'x'
	m, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	if m.lines[0] != "xmachine:" {
		t.Errorf("line = %q, want %q", m.lines[0], "xmachine:")
	}
	if !m.modified {
		t.Error("expected modified=true after insert")
	}
}

func TestBackspace(t *testing.T) {
	m := testModel()
	m.cursorRow = 0
	m.cursorCol = 3 // after "mac"

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.lines[0] != "mahine:" {
		t.Errorf("line = %q, want %q", m.lines[0], "mahine:")
	}
	if m.cursorCol != 2 {
		t.Errorf("cursorCol = %d, want 2", m.cursorCol)
	}
}

func TestEnterSplitsLine(t *testing.T) {
	m := testModel()
	m.cursorRow = 0
	m.cursorCol = 4 // after "mach"

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.lines[0] != "mach" {
		t.Errorf("line 0 = %q, want %q", m.lines[0], "mach")
	}
	if m.lines[1] != "ine:" {
		t.Errorf("line 1 = %q, want %q", m.lines[1], "ine:")
	}
	if m.cursorRow != 1 || m.cursorCol != 0 {
		t.Errorf("cursor = (%d,%d), want (1,0)", m.cursorRow, m.cursorCol)
	}
}

func TestEscExits(t *testing.T) {
	m := testModel()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg := cmd()
	if _, ok := msg.(shared.ViewChangeMsg); !ok {
		t.Errorf("expected ViewChangeMsg, got %T", msg)
	}
}

func TestModeSelector(t *testing.T) {
	m := testModel()

	// ctrl+s opens mode selector
	m, _ = m.Update(tea.KeyPressMsg{Code: 115, Mod: tea.ModCtrl})
	if !m.showModeSelector {
		t.Error("expected showModeSelector=true")
	}

	// Down selects next mode
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selectedMode != ModeReboot {
		t.Errorf("selectedMode = %d, want %d", m.selectedMode, ModeReboot)
	}

	// Esc closes mode selector
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.showModeSelector {
		t.Error("expected showModeSelector=false after Esc")
	}
}

func TestValidationPanel(t *testing.T) {
	m := testModel()
	m.validationErrors = []string{"error 1", "error 2"}
	m.showValidation = true

	// Esc dismisses
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.showValidation {
		t.Error("expected showValidation=false after Esc")
	}
}

func TestConfigBytes(t *testing.T) {
	m := testModel()
	data := m.configBytes()
	if !strings.Contains(string(data), "machine:") {
		t.Error("expected YAML content in configBytes")
	}
}

func TestView_Loading(t *testing.T) {
	m := Model{loading: true, node: "test", width: 80, height: 24}
	v := m.View()
	if v == "" {
		t.Error("expected non-empty loading view")
	}
}

func TestColorizeYAML(t *testing.T) {
	m := testModel()

	// Comment line
	result := m.colorizeYAML("# this is a comment", 80)
	if result == "" {
		t.Error("expected non-empty colorized comment")
	}

	// Key-value line
	result = m.colorizeYAML("key: value", 80)
	if result == "" {
		t.Error("expected non-empty colorized key-value")
	}
}
