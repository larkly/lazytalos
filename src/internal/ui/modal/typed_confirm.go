package modal

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// TypedConfirmAction is the result of a typed-confirmation dialog.
type TypedConfirmAction struct {
	Action     string
	ActionData any
	Confirm    bool
}

// TypedConfirmModel is a confirmation dialog that requires typing a specific
// string before the confirm action is enabled. Used for dangerous operations
// like etcd member removal.
type TypedConfirmModel struct {
	Title        string
	Body         string
	RequiredText string // user must type this exactly to enable confirm
	TypedText    string // what user has typed so far
	Action       string
	ActionData   any
	Width        int
	Height       int
}

// NewTypedConfirm creates a typed-confirmation dialog.
func NewTypedConfirm(action, title, body, requiredText string, data any) TypedConfirmModel {
	return TypedConfirmModel{
		Title:        title,
		Body:         body,
		RequiredText: requiredText,
		Action:       action,
		ActionData:   data,
	}
}

func (m TypedConfirmModel) matched() bool {
	return m.TypedText == m.RequiredText
}

func (m TypedConfirmModel) confirmMsg() tea.Cmd {
	shared.Debugf("[typed_confirm] confirmed action=%s", m.Action)
	return func() tea.Msg {
		return TypedConfirmAction{
			Action:     m.Action,
			ActionData: m.ActionData,
			Confirm:    true,
		}
	}
}

func (m TypedConfirmModel) cancelMsg() tea.Cmd {
	shared.Debugf("[typed_confirm] cancelled action=%s", m.Action)
	return func() tea.Msg {
		return TypedConfirmAction{
			Action:     m.Action,
			ActionData: m.ActionData,
			Confirm:    false,
		}
	}
}

// Update handles input for the typed-confirmation dialog.
func (m TypedConfirmModel) Update(msg tea.Msg) (TypedConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, shared.Keys.Back):
			return m, m.cancelMsg()
		case key.Matches(msg, shared.Keys.Confirm):
			if m.matched() {
				return m, m.confirmMsg()
			}
			return m, nil
		default:
			s := msg.String()
			if s == "backspace" {
				if len(m.TypedText) > 0 {
					m.TypedText = m.TypedText[:len(m.TypedText)-1]
				}
			} else if len(s) == 1 {
				m.TypedText += s
			}
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

// View renders the typed-confirmation dialog.
func (m TypedConfirmModel) View() string {
	title := shared.StyleModalTitle.Render(m.Title)

	body := m.Body

	// Input field
	prompt := fmt.Sprintf("Type %q to confirm:", m.RequiredText)
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(shared.ColorMuted).
		Padding(0, 1).
		Width(40)

	inputText := m.TypedText
	if inputText == "" {
		inputText = lipgloss.NewStyle().Foreground(shared.ColorMuted).Render(m.RequiredText)
	}
	input := inputStyle.Render(inputText)

	// Match indicator
	indicator := ""
	if m.TypedText != "" {
		if m.matched() {
			indicator = shared.StyleSuccess.Render(" ✓ match")
		} else {
			indicator = shared.StyleError.Render(" ✗ no match")
		}
	}

	// Buttons
	confirmLabel := "[ctrl+s] Confirm"
	var confirmStyle lipgloss.Style
	if m.matched() {
		confirmStyle = shared.StyleButtonSubmit
	} else {
		confirmStyle = shared.StyleButton.Foreground(shared.ColorMuted)
	}
	cancelStyle := shared.StyleButtonCancel
	buttons := confirmStyle.Render(confirmLabel) + "  " + cancelStyle.Render("[esc] Cancel")

	content := title + "\n\n" + body + "\n\n" + prompt + "\n" + input + indicator + "\n\n" + buttons
	box := shared.StyleModal.Width(60).Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
}

// SetSize updates dimensions.
func (m *TypedConfirmModel) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}
