package help

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// Model is the help overlay.
type Model struct {
	visible       bool
	scrollY       int
	width, height int
}

// New returns a new (hidden) help overlay.
func New() Model { return Model{} }

// Toggle shows or hides the overlay; scroll resets when opening.
func (m Model) Toggle() Model { m.visible = !m.visible; m.scrollY = 0; return m }

// IsVisible reports whether the overlay is shown.
func (m Model) IsVisible() bool { return m.visible }

// SetSize updates the terminal dimensions.
func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

// Update handles key input while the overlay is visible.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, shared.Keys.Up):
		if m.scrollY > 0 {
			m.scrollY--
		}
	case key.Matches(keyMsg, shared.Keys.Down):
		m.scrollY++
	default:
		m.visible = false
	}
	return m, nil
}

// View renders the help overlay centred on screen.
func (m Model) View() string {
	lines := buildHelpContent()

	maxW := 80
	if m.width < maxW {
		maxW = m.width
	}
	innerW := maxW - 4 // 2 border + 2 padding
	maxH := int(float64(m.height) * 0.9)
	visibleLines := maxH - 4 // title + footer + 2 borders

	if visibleLines < 1 {
		visibleLines = 1
	}

	// Clamp scroll
	if m.scrollY > len(lines)-visibleLines {
		m.scrollY = len(lines) - visibleLines
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}

	end := m.scrollY + visibleLines
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[m.scrollY:end]

	body := strings.Join(visible, "\n")

	title := shared.StyleHeader.Render("LAZYTALOS — Keyboard Reference")
	footer := shared.StyleMuted.Render("Any key to close")
	content := fmt.Sprintf("%s\n%s\n%s", title, body, footer)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		shared.StyleModal.Width(innerW).Render(content))
}

func buildHelpContent() []string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }
	section := func(title string) { add(""); add(shared.StyleLabel.Render(title)) }
	item := func(k, desc string) {
		add(fmt.Sprintf("  %-18s %s", shared.StyleValue.Render(k), shared.StyleMuted.Render(desc)))
	}

	section("NAVIGATION")
	item("↑ / ↓", "Navigate list")
	item("PgUp / PgDn", "Scroll page")
	item("Enter", "Detail view / confirm")
	item("Esc", "Back / close")
	item("← / →", "Previous / next tab")
	item("1–8", "Switch to tab by number")

	section("NODE ACTIONS  (Nodes tab)")
	item("Space", "Select / deselect node")
	item("A", "Select all / deselect all")
	item("Ctrl+O", "Reboot selected nodes")
	item("Ctrl+D", "Shutdown selected nodes")
	item("Ctrl+U", "Upgrade cluster (rolling)")
	item("Ctrl+X", "Reset node (dangerous)")
	item("Ctrl+E", "Edit node config")
	item("y", "Copy node IP to clipboard")
	item("Y", "Copy endpoint to clipboard")

	section("SERVICE ACTIONS")
	item("Ctrl+K", "Restart service")

	section("LOGS")
	item("F", "Toggle follow mode")

	section("ETCD")
	item("Ctrl+M", "Remove etcd member")

	section("UPGRADE")
	item("Ctrl+P", "Pause after current node")
	item("Ctrl+A", "Abort upgrade")

	section("FILTERING & SORTING")
	item("/", "Filter")
	item("s", "Cycle sort column")

	section("GLOBAL")
	item("C", "Switch cluster context")
	item("Ctrl+,", "View talosconfig")
	item("Ctrl+R", "Force refresh")
	item("?", "This help")
	item("q", "Quit")

	return lines
}
