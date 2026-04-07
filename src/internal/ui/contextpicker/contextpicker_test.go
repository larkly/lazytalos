package contextpicker

import (
	"errors"
	"testing"

	"github.com/larkly/lazytalos/internal/shared"
	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	contexts := []string{"alpha", "beta", "gamma"}
	m := New(contexts, nil)

	if len(m.contexts) != 3 {
		t.Errorf("contexts len = %d, want 3", len(m.contexts))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.err != nil {
		t.Errorf("err = %v, want nil", m.err)
	}
}

func TestNew_WithError(t *testing.T) {
	e := errors.New("no talosconfig")
	m := New(nil, e)

	if m.err == nil {
		t.Error("expected error to be stored")
	}
	if m.err.Error() != "no talosconfig" {
		t.Errorf("err = %q, want %q", m.err.Error(), "no talosconfig")
	}
}

func TestCursorDown(t *testing.T) {
	m := New([]string{"a", "b", "c"}, nil)

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}

	// Should not go past end (clamp)
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped)", m.cursor)
	}
}

func TestCursorUp(t *testing.T) {
	m := New([]string{"a", "b", "c"}, nil)

	// Should not go below 0
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m.cursor)
	}

	// Move down then up
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestEnter_SelectsContext(t *testing.T) {
	m := New([]string{"prod", "staging", "dev"}, nil)

	// Move to "staging" (index 1)
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmd()
	sel, ok := msg.(shared.ContextSelectedMsg)
	if !ok {
		t.Fatalf("expected ContextSelectedMsg, got %T", msg)
	}
	if sel.ContextName != "staging" {
		t.Errorf("ContextName = %q, want %q", sel.ContextName, "staging")
	}
}

func TestEnter_FirstItem(t *testing.T) {
	m := New([]string{"only-one"}, nil)

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if cmd == nil {
		t.Fatal("expected cmd from Enter on item 0")
	}
	msg := cmd()
	sel, ok := msg.(shared.ContextSelectedMsg)
	if !ok {
		t.Fatalf("expected ContextSelectedMsg, got %T", msg)
	}
	if sel.ContextName != "only-one" {
		t.Errorf("ContextName = %q, want %q", sel.ContextName, "only-one")
	}
}

func TestEnter_EmptyContexts(t *testing.T) {
	m := New([]string{}, nil)

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Error("expected nil cmd for empty contexts list")
	}
}

func TestQuit(t *testing.T) {
	m := New([]string{"a"}, nil)

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestWindowSize(t *testing.T) {
	m := New([]string{"a"}, nil)

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}
