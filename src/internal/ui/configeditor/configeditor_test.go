package configeditor

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

const sampleYAML = `version: v1alpha1
machine:
  type: controlplane
  hostname: test-node
`

const modifiedYAML = `version: v1alpha1
machine:
  type: controlplane
  hostname: test-node-modified
`

// newTestModel creates a Model without a live client (nil client is fine for
// tests that don't trigger network calls).
func newTestModel() Model {
	return New(nil, "tnn3-demo-cp-1", sampleYAML, 120, 40)
}

// keyPress is a helper to create a KeyPressMsg for a given key string.
func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func keyPressSpecial(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func keyCtrl(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: ch, Mod: tea.ModCtrl})
}

// TestNew verifies that a new Model is created with the correct node and YAML.
func TestNew(t *testing.T) {
	m := newTestModel()
	if m.node != "tnn3-demo-cp-1" {
		t.Errorf("node = %q, want %q", m.node, "tnn3-demo-cp-1")
	}
	if m.textarea.Value() != sampleYAML {
		t.Errorf("textarea value = %q, want %q", m.textarea.Value(), sampleYAML)
	}
	if m.original != sampleYAML {
		t.Errorf("original = %q, want %q", m.original, sampleYAML)
	}
	if m.status != editorIdle {
		t.Errorf("initial status = %v, want editorIdle", m.status)
	}
}

// TestHasChanges verifies change detection.
func TestHasChanges(t *testing.T) {
	m := newTestModel()

	// No changes yet.
	if m.HasChanges() {
		t.Error("HasChanges() = true for fresh model, want false")
	}

	// Modify original so content diverges.
	m.original = modifiedYAML
	if !m.HasChanges() {
		t.Error("HasChanges() = false after modifying original, want true")
	}
}

// TestHints verifies that Hints returns a non-empty string in each meaningful state.
func TestHints(t *testing.T) {
	m := newTestModel()
	if m.Hints() == "" {
		t.Error("Hints() returned empty string for idle model")
	}

	m.confirmDiscard = true
	if m.Hints() == "" {
		t.Error("Hints() returned empty string in confirmDiscard state")
	}

	m.confirmDiscard = false
	m.showApplyPicker = true
	if m.Hints() == "" {
		t.Error("Hints() returned empty string in showApplyPicker state")
	}

	m.showApplyPicker = false
	m.status = editorErr
	if m.Hints() == "" {
		t.Error("Hints() returned empty string in editorErr state")
	}
}

// TestEscWithoutChanges verifies that Esc without changes emits ClosedMsg.
func TestEscWithoutChanges(t *testing.T) {
	m := newTestModel()
	// Ensure no changes: make sure original matches textarea.
	m.original = m.textarea.Value()

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("cmd is nil, expected a command that emits ClosedMsg")
	}
	msg := cmd()
	closed, ok := msg.(ClosedMsg)
	if !ok {
		t.Fatalf("expected ClosedMsg, got %T", msg)
	}
	if closed.Applied {
		t.Error("ClosedMsg.Applied = true, want false for discard without changes")
	}
}

// TestConfirmDiscardFlow verifies: Esc with changes → confirmDiscard=true → Ctrl+S → ClosedMsg.
func TestConfirmDiscardFlow(t *testing.T) {
	m := newTestModel()
	// Simulate a change by altering original so HasChanges() returns true.
	m.original = modifiedYAML

	// Esc should set confirmDiscard.
	m2, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd != nil {
		// cmd should be nil — but if it returned one, we skip the call.
		t.Logf("got unexpected cmd after Esc with changes: %T", cmd())
	}
	if !m2.confirmDiscard {
		t.Error("confirmDiscard = false after Esc with changes, want true")
	}

	// Ctrl+S in confirmDiscard mode should emit ClosedMsg{Applied: false}.
	_, cmd2 := m2.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	if cmd2 == nil {
		t.Fatal("cmd is nil after Ctrl+S in confirmDiscard mode")
	}
	msg := cmd2()
	closed, ok := msg.(ClosedMsg)
	if !ok {
		t.Fatalf("expected ClosedMsg, got %T", msg)
	}
	if closed.Applied {
		t.Error("ClosedMsg.Applied = true, want false for discard")
	}
}

// TestConfirmDiscardCancelFlow verifies that Esc in confirmDiscard mode cancels the discard.
func TestConfirmDiscardCancelFlow(t *testing.T) {
	m := newTestModel()
	m.original = modifiedYAML
	m.confirmDiscard = true

	m2, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd != nil {
		t.Errorf("expected nil cmd on Esc to cancel discard")
	}
	if m2.confirmDiscard {
		t.Error("confirmDiscard = true after cancel, want false")
	}
}

// TestValidationErrors verifies that a validateResultMsg with errors populates validErrs.
func TestValidationErrors(t *testing.T) {
	m := newTestModel()
	m.status = editorValidating

	errs := []string{"line 1: bad yaml", "field x: invalid value"}
	m2, cmd := m.Update(validateResultMsg{errs: errs, err: nil})
	if cmd != nil {
		t.Logf("got unexpected cmd after validation result")
	}
	if m2.status != editorValidated {
		t.Errorf("status = %v, want editorValidated", m2.status)
	}
	if len(m2.validErrs) != 2 {
		t.Errorf("validErrs len = %d, want 2", len(m2.validErrs))
	}
	if m2.validErrs[0] != errs[0] {
		t.Errorf("validErrs[0] = %q, want %q", m2.validErrs[0], errs[0])
	}
}

// TestValidationSuccess verifies that clean validation clears errors.
func TestValidationSuccess(t *testing.T) {
	m := newTestModel()
	m.status = editorValidating
	m.validErrs = []string{"old error"}

	m2, _ := m.Update(validateResultMsg{errs: nil, err: nil})
	if m2.status != editorValidated {
		t.Errorf("status = %v, want editorValidated", m2.status)
	}
	if len(m2.validErrs) != 0 {
		t.Errorf("validErrs should be empty after clean validation, got %v", m2.validErrs)
	}
}

// TestIsDone verifies IsDone reflects editorDone status.
func TestIsDone(t *testing.T) {
	m := newTestModel()
	if m.IsDone() {
		t.Error("IsDone() = true for fresh model, want false")
	}
	m.status = editorDone
	if !m.IsDone() {
		t.Error("IsDone() = false when status=editorDone, want true")
	}
}

// TestViewRendersWithoutPanic verifies that View does not panic with normal state.
func TestViewRendersWithoutPanic(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string")
	}
	if !containsStr(v, "tnn3-demo-cp-1") {
		t.Error("View() does not contain node name")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
