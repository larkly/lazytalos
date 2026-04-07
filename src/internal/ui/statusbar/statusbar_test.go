package statusbar

import (
	"strings"
	"testing"
)

func TestRenderContainsContextName(t *testing.T) {
	m := Model{
		Context:     "tnn3-demo",
		CurrentView: "Nodes",
		Connected:   true,
		Width:       80,
		Hint:        "↑/k up  ↓/j down  q quit",
	}
	out := m.Render()
	if !strings.Contains(out, "tnn3-demo") {
		t.Errorf("Render() output does not contain context name %q\nOutput: %q", "tnn3-demo", out)
	}
}

func TestRenderContainsHint(t *testing.T) {
	m := Model{
		Context:   "my-cluster",
		Hint:      "ctrl+r refresh",
		Connected: false,
		Width:     80,
	}
	out := m.Render()
	if !strings.Contains(out, "ctrl+r refresh") {
		t.Errorf("Render() output does not contain hint text\nOutput: %q", out)
	}
}

func TestRenderDisconnectedContext(t *testing.T) {
	m := Model{
		Context:   "offline-ctx",
		Connected: false,
		Width:     80,
	}
	out := m.Render()
	if !strings.Contains(out, "offline-ctx") {
		t.Errorf("Render() output does not contain context name when disconnected\nOutput: %q", out)
	}
}
