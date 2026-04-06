package modal

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// NodeRef identifies a node for an action.
type NodeRef struct {
	ID             string
	Name           string
	IsControlPlane bool
}

// ConfirmAction is the result of a confirmation dialog.
type ConfirmAction struct {
	Action    string
	Node      string    // for single-node actions
	Nodes     []NodeRef // for bulk actions
	ServiceID string    // for "restart service" actions
	Confirm   bool
}

// ConfirmModel is a confirmation dialog.
type ConfirmModel struct {
	Action          string
	Node            string    // for single-node actions
	Nodes           []NodeRef // for bulk actions
	ServiceID       string    // for "restart service" actions
	HasControlPlane bool      // true if any selected node is a control plane node
	Body            string    // custom body text (overrides default)
	Title           string    // custom title (overrides default)
	Width           int
	Height          int
	focused         int // 0 = confirm, 1 = cancel
}

// NewConfirm creates a confirmation dialog for a single node.
func NewConfirm(action, node string) ConfirmModel {
	return ConfirmModel{
		Action:  action,
		Node:    node,
		focused: 1, // default to cancel for safety
	}
}

// NewServiceRestartConfirm creates a confirmation dialog for restarting a service on a node.
func NewServiceRestartConfirm(node, serviceID, displayName string) ConfirmModel {
	return ConfirmModel{
		Action:    "restart service",
		Node:      node,
		ServiceID: serviceID,
		Body:      fmt.Sprintf("Are you sure you want to restart service %q on node %q?", displayName, node),
		focused:   1, // default to cancel for safety
	}
}

// NewBulkConfirm creates a confirmation dialog for multiple nodes.
func NewBulkConfirm(action string, nodes []NodeRef) ConfirmModel {
	hasCP := false
	for _, n := range nodes {
		if n.IsControlPlane {
			hasCP = true
			break
		}
	}
	return ConfirmModel{
		Action:          action,
		Nodes:           nodes,
		Node:            fmt.Sprintf("%d nodes", len(nodes)),
		HasControlPlane: hasCP,
		focused:         1, // default to cancel for safety
	}
}

func (m ConfirmModel) confirmMsg() tea.Cmd {
	shared.Debugf("[confirm] confirmed action=%s node=%q", m.Action, m.Node)
	return func() tea.Msg {
		return ConfirmAction{
			Action:    m.Action,
			Node:      m.Node,
			Nodes:     m.Nodes,
			ServiceID: m.ServiceID,
			Confirm:   true,
		}
	}
}

func (m ConfirmModel) cancelMsg() tea.Cmd {
	shared.Debugf("[confirm] cancelled action=%s node=%q", m.Action, m.Node)
	return func() tea.Msg {
		return ConfirmAction{
			Action:    m.Action,
			Node:      m.Node,
			Nodes:     m.Nodes,
			ServiceID: m.ServiceID,
			Confirm:   false,
		}
	}
}

// Update handles input for the confirmation dialog.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, shared.Keys.Confirm):
			return m, m.confirmMsg()
		case key.Matches(msg, shared.Keys.Back):
			return m, m.cancelMsg()
		case key.Matches(msg, shared.Keys.Tab),
			key.Matches(msg, shared.Keys.ShiftTab),
			key.Matches(msg, shared.Keys.Left),
			key.Matches(msg, shared.Keys.Right),
			key.Matches(msg, shared.Keys.Up),
			key.Matches(msg, shared.Keys.Down):
			m.focused = 1 - m.focused
		case key.Matches(msg, shared.Keys.Enter):
			if m.focused == 0 {
				return m, m.confirmMsg()
			}
			return m, m.cancelMsg()
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

// View renders the confirmation dialog.
func (m ConfirmModel) View() string {
	titleText := m.Title
	if titleText == "" {
		titleText = fmt.Sprintf("Confirm %s", m.Action)
	}
	title := shared.StyleModalTitle.Render(titleText)

	body := m.Body
	if body == "" {
		if len(m.Nodes) > 0 {
			body = fmt.Sprintf("Are you sure you want to %s %d node(s)?", m.Action, len(m.Nodes))
		} else {
			body = fmt.Sprintf("Are you sure you want to %s node %q?", m.Action, m.Node)
		}
	}

	// Control plane warning
	warning := ""
	if m.HasControlPlane {
		warning = "\n\n" + shared.StyleWarning.Render(
			"⚠ Warning: control plane nodes selected — may cause etcd leadership re-election.",
		)
	}

	// Buttons
	confirmStyle := shared.StyleButton
	cancelStyle := shared.StyleButton
	if m.focused == 0 {
		confirmStyle = shared.StyleButtonSubmit
	} else {
		cancelStyle = shared.StyleButtonCancel
	}
	buttons := confirmStyle.Render("[ctrl+s] Confirm") + "  " + cancelStyle.Render("[esc] Cancel")

	content := title + "\n\n" + body + warning + "\n\n" + buttons
	box := shared.StyleModal.Width(60).Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
}

// SetSize updates dimensions.
func (m *ConfirmModel) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}
