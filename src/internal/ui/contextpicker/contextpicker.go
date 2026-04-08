package contextpicker

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// Model is the context picker overlay.
type Model struct {
	contexts       []string
	currentContext string
	cursor         int
	err            error
	width          int
	height         int
}

// New creates a context picker with the given context names.
func New(contexts []string, currentContext string, err error) Model {
	// Pre-select cursor to current context
	cursor := 0
	for i, c := range contexts {
		if c == currentContext {
			cursor = i
			break
		}
	}
	return Model{
		contexts:       contexts,
		currentContext: currentContext,
		cursor:         cursor,
		err:            err,
	}
}

// Init returns no initial command.
func (m Model) Init() tea.Cmd {
	shared.Debugf("[contextpicker] Init() contexts=%d err=%v", len(m.contexts), m.err)
	return nil
}

// Update handles input for the context picker.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, shared.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, shared.Keys.Down):
			if m.cursor < len(m.contexts)-1 {
				m.cursor++
			}
		case key.Matches(msg, shared.Keys.Enter):
			if len(m.contexts) > 0 {
				shared.Debugf("[contextpicker] selected context=%q", m.contexts[m.cursor])
				return m, func() tea.Msg {
					return shared.ContextSelectedMsg{ContextName: m.contexts[m.cursor]}
				}
			}
		case key.Matches(msg, shared.Keys.Quit):
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View renders the context picker.
func (m Model) View() string {
	// Error display
	if m.err != nil && len(m.contexts) == 0 {
		content := shared.StyleTitle.Render("Select a Talos context") + "\n\n" +
			lipgloss.NewStyle().Foreground(shared.ColorError).Render(m.err.Error()) + "\n\n" +
			shared.StyleMuted.Render("Check your talosconfig file") + "\n" +
			shared.StyleMuted.Render("Press q to quit")

		box := shared.StyleErrorModal.Width(60).Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	title := shared.StyleTitle.Render("Select a Talos context")

	// Error banner (non-fatal, contexts still available)
	errBanner := ""
	if m.err != nil {
		errBanner = "\n" + lipgloss.NewStyle().Foreground(shared.ColorError).Render(m.err.Error()) + "\n"
	}

	// Empty state
	if len(m.contexts) == 0 {
		content := title + "\n\n" +
			shared.StyleMuted.Render("No contexts found in talosconfig") + "\n\n" +
			shared.StyleMuted.Render("Press q to quit")
		box := shared.StyleModal.Width(40).Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	items := ""
	for i, name := range m.contexts {
		label := name
		if name == m.currentContext {
			label = name + " (current)"
		}
		if i == m.cursor {
			items += shared.StyleSelected.Render(fmt.Sprintf("  %s  ", label)) + "\n"
		} else if name == m.currentContext {
			items += lipgloss.NewStyle().Foreground(shared.ColorPrimary).Render(fmt.Sprintf("  %s  ", label)) + "\n"
		} else {
			items += shared.StyleMuted.Render(fmt.Sprintf("  %s  ", label)) + "\n"
		}
	}

	hint := shared.StyleMuted.Render("j/k navigate | enter select | q quit")

	content := title + errBanner + "\n" + items + "\n" + hint
	box := shared.StyleModal.Width(40).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// SetSize updates the dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Hints returns the hint text for the status bar.
func (m Model) Hints() string {
	return "Select a context to connect"
}
