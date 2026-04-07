package configview

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New("/some/path")
	if m.IsVisible() {
		t.Error("New() model should not be visible")
	}
	if m.configPath != "/some/path" {
		t.Errorf("configPath = %q, want %q", m.configPath, "/some/path")
	}
}

func TestToggle(t *testing.T) {
	// Toggle with invalid path: visible but shows error (no panic)
	m := New("/nonexistent/talosconfig")
	m = m.Toggle()
	if !m.IsVisible() {
		t.Error("after Toggle() model should be visible")
	}
	m = m.Toggle()
	if m.IsVisible() {
		t.Error("after second Toggle() model should be hidden")
	}
}

func TestEscEmitsClosedMsg(t *testing.T) {
	m := New("/nonexistent/talosconfig")
	m = m.Toggle() // make visible

	m2, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m2.IsVisible() {
		t.Error("after Esc, model should be hidden")
	}
	if cmd == nil {
		t.Fatal("expected a cmd from Esc key")
	}
	msg := cmd()
	if _, ok := msg.(ClosedMsg); !ok {
		t.Errorf("expected ClosedMsg, got %T", msg)
	}
}

func TestLoad_InvalidPath(t *testing.T) {
	// Should not panic; just store error
	m := New("/nonexistent/path/talosconfig")
	m = m.load()
	if m.err == "" {
		t.Error("expected error string for invalid path")
	}
	if len(m.contexts) != 0 {
		t.Error("expected no contexts for invalid path")
	}
}

func TestCADigest(t *testing.T) {
	// Encode some bytes as base64 and verify digest
	data := []byte("test-ca-cert-data")
	encoded := base64.StdEncoding.EncodeToString(data)

	digest := caDigest(encoded)
	if len(digest) != 16 {
		t.Errorf("caDigest length = %d, want 16", len(digest))
	}

	// Verify it's the first 16 hex chars of sha256
	sum := sha256.Sum256(data)
	expected := fmt.Sprintf("%x", sum[:])[:16]
	if digest != expected {
		t.Errorf("caDigest = %q, want %q", digest, expected)
	}
}

func TestCADigest_Empty(t *testing.T) {
	digest := caDigest("")
	if digest != "" {
		t.Errorf("caDigest(\"\") = %q, want \"\"", digest)
	}
}

func TestUpdateWhenHidden(t *testing.T) {
	m := New("/some/path")
	// Hidden, key should be ignored
	m2, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m2.IsVisible() {
		t.Error("hidden model should remain hidden")
	}
	if cmd != nil {
		t.Error("expected nil cmd when model is hidden")
	}
}
