package modal

import (
	"testing"

	"charm.land/bubbletea/v2"
)

func TestTypedConfirm_ConfirmBlockedUntilMatch(t *testing.T) {
	m := NewTypedConfirm("remove", "Remove Member", "Are you sure?", "abc123", nil)
	m.Width = 80
	m.Height = 24

	// ctrl+s should not confirm when text doesn't match
	m2, cmd := m.Update(tea.KeyPressMsg{Code: 115, Mod: tea.ModCtrl}) // ctrl+s
	if cmd != nil {
		t.Error("expected no command when text doesn't match")
	}

	// Type matching text
	for _, ch := range "abc123" {
		m2, _ = m2.Update(tea.KeyPressMsg{Code: rune(ch)})
	}
	if !m2.matched() {
		t.Error("expected match after typing required text")
	}

	// Now ctrl+s should work
	_, cmd = m2.Update(tea.KeyPressMsg{Code: 115, Mod: tea.ModCtrl}) // ctrl+s
	if cmd == nil {
		t.Error("expected command when text matches")
	}
	msg := cmd()
	action, ok := msg.(TypedConfirmAction)
	if !ok {
		t.Fatalf("expected TypedConfirmAction, got %T", msg)
	}
	if !action.Confirm {
		t.Error("expected Confirm=true")
	}
}

func TestTypedConfirm_Cancel(t *testing.T) {
	m := NewTypedConfirm("remove", "Remove", "Sure?", "abc", nil)
	m.Width = 80
	m.Height = 24

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	msg := cmd()
	action, ok := msg.(TypedConfirmAction)
	if !ok {
		t.Fatalf("expected TypedConfirmAction, got %T", msg)
	}
	if action.Confirm {
		t.Error("expected Confirm=false on cancel")
	}
}

func TestTypedConfirm_Backspace(t *testing.T) {
	m := NewTypedConfirm("remove", "Remove", "Sure?", "ab", nil)
	m.Width = 80
	m.Height = 24

	// Type "abc"
	for _, ch := range "abc" {
		m, _ = m.Update(tea.KeyPressMsg{Code: rune(ch)})
	}
	if m.TypedText != "abc" {
		t.Errorf("typed = %q, want %q", m.TypedText, "abc")
	}

	// Backspace once
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.TypedText != "ab" {
		t.Errorf("after backspace: typed = %q, want %q", m.TypedText, "ab")
	}
	if !m.matched() {
		t.Error("expected match after backspace to 'ab'")
	}
}

func TestTypedConfirm_ActionData(t *testing.T) {
	data := uint64(12345)
	m := NewTypedConfirm("remove", "Remove", "Sure?", "x", data)
	m.Width = 80
	m.Height = 24

	m, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 115, Mod: tea.ModCtrl}) // ctrl+s
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	action := msg.(TypedConfirmAction)
	if action.ActionData.(uint64) != 12345 {
		t.Errorf("ActionData = %v, want 12345", action.ActionData)
	}
}
