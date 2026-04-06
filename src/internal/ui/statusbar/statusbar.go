// Package statusbar provides a bottom status bar component for lazytalos.
package statusbar

import (
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// Model holds the state for the status bar.
type Model struct {
	CurrentView string
	Hint        string
	Context     string // cluster context name, e.g. "tnn3-demo"
	Connected   bool
	Width       int
}

// Render produces a full-width status bar string.
func (m Model) Render() string {
	// Left side: context + current view
	var ctxStyle lipgloss.Style
	if m.Connected {
		ctxStyle = lipgloss.NewStyle().
			Foreground(shared.ColorSuccess).
			Bold(true)
	} else {
		ctxStyle = lipgloss.NewStyle().
			Foreground(shared.ColorMuted)
	}

	ctx := m.Context
	if ctx == "" {
		ctx = "(no context)"
	}
	left := ctxStyle.Render(ctx)
	if m.CurrentView != "" {
		left += "  " + lipgloss.NewStyle().Foreground(shared.ColorFg).Render(m.CurrentView)
	}

	// Right side: hint text
	right := lipgloss.NewStyle().
		Foreground(shared.ColorMuted).
		Render(m.Hint)

	// Compute padding to fill the width
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	space := m.Width - leftLen - rightLen - 2 // -2 for the PaddingLeft+Right on the bar
	if space < 1 {
		space = 1
	}
	padding := lipgloss.NewStyle().Width(space).Render("")

	content := left + padding + right
	return shared.StyleStatusBar.Width(m.Width).Render(content)
}
