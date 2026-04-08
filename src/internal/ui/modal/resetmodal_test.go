package modal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewResetModal(t *testing.T) {
	m := NewResetModal("cp-1", false, 80, 24)
	if m.node != "cp-1" {
		t.Errorf("expected node cp-1, got %q", m.node)
	}
	if m.step != ResetStepTyped {
		t.Error("expected step to be ResetStepTyped initially")
	}
	if m.modeIdx != 0 {
		t.Errorf("expected modeIdx 0 (graceful), got %d", m.modeIdx)
	}
}

func TestTypedInput_WrongHostname_NoAdvance(t *testing.T) {
	m := NewResetModal("cp-1", false, 80, 24)

	// Type wrong hostname
	for _, c := range "wrong" {
		m, _ = m.Update(tea.KeyPressMsg{Code: rune(c), Text: string(c)})
	}
	// Press enter
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.step != ResetStepTyped {
		t.Error("expected step to remain ResetStepTyped after wrong hostname + enter")
	}
}

func TestTypedInput_CorrectHostname_Advances(t *testing.T) {
	m := NewResetModal("cp-1", false, 80, 24)

	// Type correct hostname
	for _, c := range "cp-1" {
		m, _ = m.Update(tea.KeyPressMsg{Code: rune(c), Text: string(c)})
	}
	// Press enter
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.step != ResetStepMode {
		t.Errorf("expected step ResetStepMode after correct hostname + enter, got %d", m.step)
	}
}

func TestResetConfirmedMsg(t *testing.T) {
	m := NewResetModal("cp-1", false, 80, 24)
	m.step = ResetStepMode // jump to step 2 directly (graceful=true, modeIdx=0)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected a command after ctrl+s in step 2")
	}
	msg := cmd()
	confirmed, ok := msg.(ResetConfirmedMsg)
	if !ok {
		t.Fatalf("expected ResetConfirmedMsg, got %T", msg)
	}
	if confirmed.Node != "cp-1" {
		t.Errorf("expected node cp-1, got %q", confirmed.Node)
	}
	if !confirmed.Graceful {
		t.Error("expected Graceful=true when modeIdx=0")
	}
}

func TestResetCancelledMsg(t *testing.T) {
	m := NewResetModal("cp-1", false, 80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a command after esc in step 1")
	}
	msg := cmd()
	if _, ok := msg.(ResetCancelledMsg); !ok {
		t.Fatalf("expected ResetCancelledMsg, got %T", msg)
	}
}
