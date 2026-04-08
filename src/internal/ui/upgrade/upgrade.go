// Package upgrade provides a 6-step cluster upgrade wizard overlay.
package upgrade

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	upgradelib "github.com/larkly/lazytalos/internal/upgrade"
)

// ClosedMsg is emitted when the wizard closes.
type ClosedMsg struct{ Completed bool }

// stepName returns the human-readable name for each step.
var stepNames = [6]string{
	"Node Selection",
	"Image",
	"Options",
	"Order Preview",
	"Confirm",
	"Executing",
}

// Model is the upgrade wizard model.
type Model struct {
	client      *talos.Client
	clusterName string
	step        int // 0-5
	state       upgradelib.State

	imageInput   textinput.Model
	confirmInput textinput.Model

	nodeCheckboxes []bool
	nodes          []cluster.NodeInfo
	cursor         int
	preserve       bool
	stage          bool

	width, height int
	errMsg        string
}

// New creates a new upgrade wizard.
func New(client *talos.Client, clusterName string, nodes []cluster.NodeInfo, w, h int) Model {
	cb := make([]bool, len(nodes))
	for i := range cb {
		cb[i] = true
	}

	img := textinput.New()
	img.Placeholder = "ghcr.io/siderolabs/installer:v1.x.y"
	img.CharLimit = 256
	img.SetWidth(60)

	conf := textinput.New()
	conf.Placeholder = clusterName
	conf.CharLimit = 256
	conf.SetWidth(60)

	return Model{
		client:         client,
		clusterName:    clusterName,
		nodes:          nodes,
		nodeCheckboxes: cb,
		imageInput:     img,
		confirmInput:   conf,
		width:          w,
		height:         h,
	}
}

// SetSize updates view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Hints returns keyboard hints for the status bar.
func (m Model) Hints() string {
	switch m.step {
	case 0:
		return "Space=toggle  Enter=next  Esc=cancel"
	case 1:
		return "Enter=next  Esc=back"
	case 2:
		return "Space=toggle  Enter=next  Esc=back"
	case 3:
		return "Enter=next  Esc=back"
	case 4:
		return "Enter=confirm  Esc=back"
	case 5:
		hints := "Ctrl+P=pause  Ctrl+A=abort"
		if m.state.Paused {
			hints = "PAUSED — Ctrl+P=resume  Ctrl+A=abort"
		}
		return hints
	}
	return ""
}

// selectedNodes returns the nodes that are checked.
func (m Model) selectedNodes() []cluster.NodeInfo {
	var sel []cluster.NodeInfo
	for i, checked := range m.nodeCheckboxes {
		if checked {
			sel = append(sel, m.nodes[i])
		}
	}
	return sel
}

func (m Model) selectedCount() int {
	n := 0
	for _, c := range m.nodeCheckboxes {
		if c {
			n++
		}
	}
	return n
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case shared.NodeUpgradedMsg:
		return m.handleNodeUpgraded(msg)
	case shared.NodeHealthyMsg:
		return m.handleNodeHealthy(msg)
	case shared.NodeHealthErrMsg:
		return m.handleNodeHealthErr(msg)
	case shared.NodeUpgradeErrMsg:
		return m.handleNodeUpgradeErr(msg)
	case pollHealthTickMsg:
		cmd := upgradelib.PollHealth(context.Background(), m.client, msg.index, msg.hostname)
		return m, cmd
	}

	// Propagate to focused text input.
	var cmd tea.Cmd
	switch m.step {
	case 1:
		m.imageInput, cmd = m.imageInput.Update(msg)
	case 4:
		m.confirmInput, cmd = m.confirmInput.Update(msg)
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Step 5 has its own key handling (pause/abort only).
	if m.step == 5 {
		return m.handleKeyStep5(msg)
	}

	switch {
	case key.Matches(msg, shared.Keys.Back):
		return m.handleEsc()
	case key.Matches(msg, shared.Keys.Enter):
		return m.handleEnter()
	}

	// Step-specific keys.
	switch m.step {
	case 0:
		return m.handleKeyStep0(msg)
	case 1:
		var cmd tea.Cmd
		m.imageInput, cmd = m.imageInput.Update(msg)
		return m, cmd
	case 2:
		return m.handleKeyStep2(msg)
	case 4:
		var cmd tea.Cmd
		m.confirmInput, cmd = m.confirmInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleEsc() (Model, tea.Cmd) {
	if m.step == 0 {
		return m, func() tea.Msg { return ClosedMsg{Completed: false} }
	}
	m.step--
	// Re-focus inputs when stepping back.
	switch m.step {
	case 1:
		m.imageInput.Focus()
	case 4:
		m.confirmInput.Focus()
	}
	return m, nil
}

func (m Model) handleEnter() (Model, tea.Cmd) {
	switch m.step {
	case 0:
		if m.selectedCount() == 0 {
			m.errMsg = "Select at least one node"
			return m, nil
		}
		m.errMsg = ""
		m.step = 1
		m.imageInput.Focus()
		return m, nil
	case 1:
		if strings.TrimSpace(m.imageInput.Value()) == "" {
			m.errMsg = "Image cannot be empty"
			return m, nil
		}
		m.errMsg = ""
		m.step = 2
		m.imageInput.Blur()
		return m, nil
	case 2:
		m.step = 3
		// Build state for order preview.
		sel := m.selectedNodes()
		opts := upgradelib.Options{
			Image:    m.imageInput.Value(),
			Preserve: m.preserve,
			Stage:    m.stage,
		}
		m.state = upgradelib.NewState(sel, opts)
		return m, nil
	case 3:
		m.step = 4
		m.confirmInput.SetValue("")
		m.confirmInput.Focus()
		return m, nil
	case 4:
		if m.confirmInput.Value() != m.clusterName {
			m.errMsg = fmt.Sprintf("Type %q to confirm", m.clusterName)
			return m, nil
		}
		m.errMsg = ""
		m.confirmInput.Blur()
		return m.startExecution()
	}
	return m, nil
}

func (m Model) startExecution() (Model, tea.Cmd) {
	m.step = 5
	m.state.Active = 0
	if len(m.state.Nodes) > 0 {
		m.state.Nodes[0].Phase = upgradelib.NodePhaseUpgrading
		m.state.Nodes[0].StartedAt = time.Now()
	}
	cmd := upgradelib.StartNode(context.Background(), m.client, m.state, 0)
	return m, cmd
}

func (m Model) handleKeyStep0(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, shared.Keys.Down):
		if m.cursor < len(m.nodes)-1 {
			m.cursor++
		}
	case key.Matches(msg, shared.Keys.Select):
		if m.cursor < len(m.nodeCheckboxes) {
			m.nodeCheckboxes[m.cursor] = !m.nodeCheckboxes[m.cursor]
		}
	}
	return m, nil
}

func (m Model) handleKeyStep2(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, shared.Keys.Down):
		if m.cursor < 1 {
			m.cursor++
		}
	case key.Matches(msg, shared.Keys.Select):
		switch m.cursor {
		case 0:
			m.preserve = !m.preserve
		case 1:
			m.stage = !m.stage
		}
	}
	return m, nil
}

func (m Model) handleKeyStep5(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.PauseUpgrade):
		m.state.Paused = !m.state.Paused
		return m, nil
	case key.Matches(msg, shared.Keys.AbortUpgrade):
		m.state.Aborted = true
		return m, func() tea.Msg { return ClosedMsg{Completed: false} }
	}
	return m, nil
}

func (m Model) handleNodeUpgraded(msg shared.NodeUpgradedMsg) (Model, tea.Cmd) {
	idx := msg.Index
	if idx < 0 || idx >= len(m.state.Nodes) {
		return m, nil
	}
	m.state.Nodes[idx].Phase = upgradelib.NodePhaseWaitingHealth
	hostname := m.state.Nodes[idx].Hostname
	cmd := upgradelib.PollHealth(context.Background(), m.client, idx, hostname)
	return m, cmd
}

func (m Model) handleNodeHealthy(msg shared.NodeHealthyMsg) (Model, tea.Cmd) {
	idx := msg.Index
	if idx < 0 || idx >= len(m.state.Nodes) {
		return m, nil
	}
	m.state.Nodes[idx].Phase = upgradelib.NodePhaseDone
	m.state.Nodes[idx].FinishedAt = time.Now()

	// Check if all done.
	allDone := true
	for _, ns := range m.state.Nodes {
		if ns.Phase != upgradelib.NodePhaseDone {
			allDone = false
			break
		}
	}
	if allDone {
		return m, func() tea.Msg { return ClosedMsg{Completed: true} }
	}

	// If paused, don't start next node.
	if m.state.Paused {
		return m, nil
	}

	// Start next node.
	next := idx + 1
	if next < len(m.state.Nodes) {
		m.state.Active = next
		m.state.Nodes[next].Phase = upgradelib.NodePhaseUpgrading
		m.state.Nodes[next].StartedAt = time.Now()
		cmd := upgradelib.StartNode(context.Background(), m.client, m.state, next)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleNodeHealthErr(msg shared.NodeHealthErrMsg) (Model, tea.Cmd) {
	idx := msg.Index
	if idx < 0 || idx >= len(m.state.Nodes) {
		return m, nil
	}
	// Retry after a short delay.
	hostname := m.state.Nodes[idx].Hostname
	return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return pollHealthTickMsg{index: idx, hostname: hostname}
	})
}

// pollHealthTickMsg triggers a health poll retry after delay.
type pollHealthTickMsg struct {
	index    int
	hostname string
}

func (m Model) handleNodeUpgradeErr(msg shared.NodeUpgradeErrMsg) (Model, tea.Cmd) {
	idx := msg.Index
	if idx < 0 || idx >= len(m.state.Nodes) {
		return m, nil
	}
	m.state.Nodes[idx].Phase = upgradelib.NodePhaseError
	m.state.Nodes[idx].ErrMsg = msg.Err.Error()
	m.state.Paused = true
	return m, nil
}

// View renders the full-screen upgrade wizard overlay.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	// Title bar.
	title := fmt.Sprintf(" CLUSTER UPGRADE — Step %d/5: %s", m.step+1, stepNames[m.step])
	sb.WriteString(shared.StyleHeader.Render(title))
	sb.WriteString("\n\n")

	// Content per step.
	switch m.step {
	case 0:
		sb.WriteString(m.viewNodeSelection())
	case 1:
		sb.WriteString(m.viewImageInput())
	case 2:
		sb.WriteString(m.viewOptions())
	case 3:
		sb.WriteString(m.viewOrderPreview())
	case 4:
		sb.WriteString(m.viewConfirm())
	case 5:
		sb.WriteString(m.viewExecuting())
	}

	// Error message.
	if m.errMsg != "" && m.step != 5 {
		sb.WriteString("\n")
		sb.WriteString(shared.StyleError.Render("  " + m.errMsg))
		sb.WriteString("\n")
	}

	// Hints footer.
	sb.WriteString("\n")
	sb.WriteString(shared.StyleMuted.Render(" " + m.Hints()))
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) viewNodeSelection() string {
	var sb strings.Builder
	sb.WriteString("  Select nodes to upgrade:\n\n")
	for i, n := range m.nodes {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		check := "[ ]"
		if m.nodeCheckboxes[i] {
			check = "[x]"
		}
		role := "worker"
		if n.IsControlPlane() {
			role = "controlplane"
		}
		line := fmt.Sprintf("  %s%s %s (%s)", cursor, check, n.Hostname, role)
		if i == m.cursor {
			sb.WriteString(shared.StyleSelected.Render(line))
		} else {
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) viewImageInput() string {
	var sb strings.Builder
	sb.WriteString("  Installer image:\n\n")
	sb.WriteString("  ")
	sb.WriteString(m.imageInput.View())
	sb.WriteString("\n")
	return sb.String()
}

func (m Model) viewOptions() string {
	var sb strings.Builder
	sb.WriteString("  Upgrade options:\n\n")

	options := []struct {
		label   string
		checked bool
	}{
		{"Preserve ephemeral data", m.preserve},
		{"Stage (apply on reboot)", m.stage},
	}

	for i, opt := range options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		check := "[ ]"
		if opt.checked {
			check = "[x]"
		}
		line := fmt.Sprintf("  %s%s %s", cursor, check, opt.label)
		if i == m.cursor {
			sb.WriteString(shared.StyleSelected.Render(line))
		} else {
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) viewOrderPreview() string {
	var sb strings.Builder
	sb.WriteString("  Upgrade order (workers first, then control planes):\n\n")
	for i, ns := range m.state.Nodes {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ns.Hostname))
	}
	return sb.String()
}

func (m Model) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Type %q to confirm upgrade:\n\n", m.clusterName))
	sb.WriteString("  ")
	sb.WriteString(m.confirmInput.View())
	sb.WriteString("\n")
	return sb.String()
}

func (m Model) viewExecuting() string {
	var sb strings.Builder
	sb.WriteString("  Upgrade progress:\n\n")
	for _, ns := range m.state.Nodes {
		var status string
		switch ns.Phase {
		case upgradelib.NodePhasePending:
			status = shared.StyleMuted.Render(fmt.Sprintf("  [    ] %s waiting", ns.Hostname))
		case upgradelib.NodePhaseUpgrading:
			status = shared.StyleWarning.Render(fmt.Sprintf("  [ACTIVE] %s upgrading...", ns.Hostname))
		case upgradelib.NodePhaseWaitingHealth:
			status = shared.StyleWarning.Render(fmt.Sprintf("  [WAIT] %s waiting for health...", ns.Hostname))
		case upgradelib.NodePhaseDone:
			status = shared.StyleSuccess.Render(fmt.Sprintf("  [DONE] %s ✓", ns.Hostname))
		case upgradelib.NodePhaseError:
			status = shared.StyleError.Render(fmt.Sprintf("  [ERROR] %s: %s", ns.Hostname, ns.ErrMsg))
		}
		sb.WriteString(status)
		sb.WriteString("\n")
	}

	if m.state.Paused {
		sb.WriteString("\n")
		sb.WriteString(shared.StyleWarning.Render("  ⏸ PAUSED — Press Ctrl+P to resume"))
		sb.WriteString("\n")
	}

	return sb.String()
}
