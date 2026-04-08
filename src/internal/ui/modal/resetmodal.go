package modal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// ResetStep tracks which step of the two-step confirmation we are on.
type ResetStep int

const (
	ResetStepTyped ResetStep = iota // step 1: type node hostname
	ResetStepMode                   // step 2: choose graceful/no-graceful
)

// ResetModal is the two-step confirmation modal for node reset.
type ResetModal struct {
	node           string
	isControlPlane bool
	step           ResetStep
	typedInput     string
	modeIdx        int // 0=graceful (default), 1=no-graceful
	width          int
	height         int
}

// ResetConfirmedMsg is emitted when both steps complete.
type ResetConfirmedMsg struct {
	Node     string
	Graceful bool
}

// ResetCancelledMsg is emitted when the user cancels.
type ResetCancelledMsg struct{}

// NewResetModal creates a new ResetModal for the given node.
func NewResetModal(node string, isCP bool, width, height int) ResetModal {
	return ResetModal{node: node, isControlPlane: isCP, width: width, height: height}
}

// IsActive always returns true (the modal is active when instantiated).
func (m ResetModal) IsActive() bool { return true }

// SetSize updates the modal dimensions.
func (m *ResetModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles key input for both steps.
func (m ResetModal) Update(msg tea.Msg) (ResetModal, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch m.step {
	case ResetStepTyped:
		switch {
		case keyMsg.Code == tea.KeyEnter:
			if m.typedInput == m.node {
				m.step = ResetStepMode
			}
		case keyMsg.Code == tea.KeyBackspace || keyMsg.Code == tea.KeyDelete:
			runes := []rune(m.typedInput)
			if len(runes) > 0 {
				m.typedInput = string(runes[:len(runes)-1])
			}
		case keyMsg.Code == tea.KeyEsc:
			return m, func() tea.Msg { return ResetCancelledMsg{} }
		default:
			if keyMsg.Text != "" {
				m.typedInput += keyMsg.Text
			}
		}

	case ResetStepMode:
		switch {
		case key.Matches(keyMsg, shared.Keys.Up):
			if m.modeIdx > 0 {
				m.modeIdx--
			}
		case key.Matches(keyMsg, shared.Keys.Down):
			if m.modeIdx < 1 {
				m.modeIdx++
			}
		case key.Matches(keyMsg, shared.Keys.Confirm):
			graceful := m.modeIdx == 0
			node := m.node
			return m, func() tea.Msg { return ResetConfirmedMsg{Node: node, Graceful: graceful} }
		case keyMsg.Code == tea.KeyEsc:
			m.step = ResetStepTyped
			m.typedInput = ""
		}
	}
	return m, nil
}

// View renders the modal.
func (m ResetModal) View() string {
	var content string

	title := shared.StyleModalTitle.Render("Reset Node")
	warning := shared.StyleError.Render("WARNING: This will reset the node and erase all data!")
	if m.isControlPlane {
		warning += "\n" + shared.StyleError.Render("DANGER: This is a control plane node! Resetting may cause etcd quorum loss and destroy the cluster.")
	}

	switch m.step {
	case ResetStepTyped:
		prompt := fmt.Sprintf("Type %q to confirm:", m.node)
		typed := shared.StyleValue.Render(m.typedInput) + "_"
		hint := shared.StyleMuted.Render("esc:cancel  enter:next")
		content = strings.Join([]string{
			title,
			"",
			warning,
			"",
			prompt,
			"> " + typed,
			"",
			hint,
		}, "\n")

	case ResetStepMode:
		gracefulLine := "  Graceful reset (recommended)"
		harshLine := "  Immediate reset (no graceful shutdown)"
		if m.modeIdx == 0 {
			gracefulLine = shared.StyleSelected.Render("> Graceful reset (recommended)")
			harshLine = "  Immediate reset (no graceful shutdown)"
		} else {
			gracefulLine = "  Graceful reset (recommended)"
			harshLine = shared.StyleSelected.Render("> Immediate reset (no graceful shutdown)")
		}
		hint := shared.StyleMuted.Render("↑/↓:select  ctrl+s:confirm  esc:back")
		content = strings.Join([]string{
			title,
			"",
			warning,
			"",
			shared.StyleLabel.Render("Select reset mode:"),
			gracefulLine,
			harshLine,
			"",
			hint,
		}, "\n")
	}

	box := shared.StyleModal.Width(60).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
