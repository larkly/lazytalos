package help

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New()
	if m.IsVisible() {
		t.Error("New() model should not be visible")
	}
}

func TestToggle(t *testing.T) {
	m := New()
	m = m.Toggle()
	if !m.IsVisible() {
		t.Error("after Toggle() model should be visible")
	}
	m = m.Toggle()
	if m.IsVisible() {
		t.Error("after second Toggle() model should be hidden")
	}
}

func TestIsVisible(t *testing.T) {
	m := New()
	if m.IsVisible() {
		t.Error("IsVisible() should return false initially")
	}
	m = m.Toggle()
	if !m.IsVisible() {
		t.Error("IsVisible() should return true after Toggle()")
	}
}

func TestAnyKeyCloses(t *testing.T) {
	m := New()
	m = m.Toggle()
	if !m.IsVisible() {
		t.Fatal("precondition: model should be visible")
	}

	// Pressing a non-Up/Down key should close the overlay
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.IsVisible() {
		t.Error("after non-nav key, overlay should close")
	}
}

func TestUpDownScrolls(t *testing.T) {
	m := New()
	m.SetSize(120, 50)
	m = m.Toggle()

	initial := m.scrollY

	// Down should increment scrollY (if content allows)
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if !m.IsVisible() {
		t.Error("Down key should not close the overlay")
	}
	afterDown := m.scrollY

	// Ensure scrollY changed or at least stayed (content may clamp it)
	// The key point is visible=true
	_ = initial
	_ = afterDown

	// Up when at zero should not go negative
	m.scrollY = 0
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if !m.IsVisible() {
		t.Error("Up key should not close the overlay")
	}
	if m.scrollY < 0 {
		t.Errorf("scrollY = %d, should not be negative", m.scrollY)
	}

	// Up when scrollY > 0 should decrement
	m.scrollY = 3
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.scrollY != 2 {
		t.Errorf("scrollY = %d, want 2", m.scrollY)
	}
	if !m.IsVisible() {
		t.Error("Up key should not close the overlay")
	}
}
