package modal

import (
	"errors"
	"testing"

	"charm.land/bubbletea/v2"
)

func TestErrorEnterDismisses(t *testing.T) {
	m := NewError("test context", errors.New("something went wrong"))

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command after Enter, got nil")
	}
	result := cmd()
	if _, ok := result.(ErrorDismissedMsg); !ok {
		t.Fatalf("expected ErrorDismissedMsg, got %T", result)
	}
}

func TestErrorEscDismisses(t *testing.T) {
	m := NewError("test context", errors.New("something went wrong"))

	msg := tea.KeyPressMsg{Code: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command after Esc, got nil")
	}
	result := cmd()
	if _, ok := result.(ErrorDismissedMsg); !ok {
		t.Fatalf("expected ErrorDismissedMsg, got %T", result)
	}
}
