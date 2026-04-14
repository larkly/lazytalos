// Package configeditor provides a full-screen overlay for editing a Talos
// machine configuration YAML in-place.
package configeditor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/config"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// ClosedMsg is emitted when the config editor is exited (discarded or after apply).
type ClosedMsg struct{ Applied bool }

// editorStatus tracks the current workflow state.
type editorStatus int

const (
	editorIdle       editorStatus = iota
	editorValidating              // Ctrl+V was pressed, waiting for result
	editorValidated               // validation completed (errors may or may not be present)
	editorApplying                // apply in progress
	editorDone                    // apply succeeded
	editorErr                     // unrecoverable error
)

// validateResultMsg carries the result of an async ValidateConfig call.
type validateResultMsg struct {
	errs            []string
	err             error
	openPickerOnClean bool // when true, open apply picker if no errors
}

// applyResultMsg carries the result of an async ApplyConfig call.
type applyResultMsg struct {
	err error
}

// applyMode labels for the picker.
var applyModeLabels = [3]string{
	"Reboot (causes node reboot)",
	"No-reboot (apply without reboot)",
	"Staged (apply on next reboot)",
}

// applyModes maps picker index to config.ApplyMode.
var applyModes = [3]config.ApplyMode{
	config.ApplyModeReboot,
	config.ApplyModeNoReboot,
	config.ApplyModeStaged,
}

// Model is the config editor model.
type Model struct {
	node   string
	client *talos.Client

	textarea textarea.Model
	original string

	validErrs []string
	status    editorStatus
	errMsg    string

	showApplyPicker bool
	applyPickerIdx  int // 0=Reboot, 1=No-reboot, 2=Staged

	confirmDiscard bool

	width, height int
}

// New creates a config editor model pre-populated with the given YAML.
func New(client *talos.Client, node, yaml string, width, height int) Model {
	ta := textarea.New()
	ta.SetValue(yaml)

	// Disable the default ctrl+v (paste) so we can use it for validation.
	km := ta.KeyMap
	km.Paste.SetEnabled(false)
	ta.KeyMap = km

	// Disable ctrl+k (delete-after-cursor) because it conflicts with Settings overlay.
	ta.KeyMap.DeleteAfterCursor.SetEnabled(false)

	ta.ShowLineNumbers = true
	ta.CharLimit = 0 // no limit

	m := Model{
		node:     node,
		client:   client,
		textarea: ta,
		original: yaml,
		width:    width,
		height:   height,
	}
	m.resize()
	_ = m.textarea.Focus()
	return m
}

// SetSize updates view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.resize()
}

func (m *Model) resize() {
	taHeight := m.textareaHeight()
	if taHeight < 1 {
		taHeight = 1
	}
	w := m.width
	if w < 10 {
		w = 10
	}
	m.textarea.SetWidth(w)
	m.textarea.SetHeight(taHeight)
}

// textareaHeight returns the number of rows available to the textarea.
// Layout: 1 header + textarea + errorPanel (up to 5) + 1 hints = height - 2 baseline.
func (m *Model) textareaHeight() int {
	reserved := 2 // header + hints
	errLines := len(m.validErrs)
	if errLines > 5 {
		errLines = 5
	}
	h := m.height - reserved - errLines
	if h < 3 {
		h = 3
	}
	return h
}

// HasChanges returns true when the current textarea content differs from the original YAML.
func (m Model) HasChanges() bool {
	return m.textarea.Value() != m.original
}

// IsDone returns true when the apply succeeded.
func (m Model) IsDone() bool {
	return m.status == editorDone
}

// Hints returns keyboard hint text for the status bar.
func (m Model) Hints() string {
	switch {
	case m.confirmDiscard:
		return "Ctrl+S=discard and close  Esc=cancel"
	case m.showApplyPicker:
		return "↑↓=select mode  Enter=apply  Esc=cancel"
	case m.status == editorValidating:
		return "validating…"
	case m.status == editorApplying:
		return "applying…"
	case m.status == editorErr:
		return "error — Esc to close"
	default:
		return "Ctrl+V=validate  Ctrl+S=apply  Esc=discard"
	}
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case validateResultMsg:
		return m.handleValidateResult(msg)
	case applyResultMsg:
		return m.handleApplyResult(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Propagate unhandled messages to textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) handleValidateResult(msg validateResultMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.status = editorErr
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.validErrs = msg.errs
	m.status = editorValidated
	m.resize() // error panel may now occupy rows
	// If triggered via Ctrl+S and config is clean, open apply picker immediately.
	if msg.openPickerOnClean && len(msg.errs) == 0 {
		m.showApplyPicker = true
	}
	return m, nil
}

func (m Model) handleApplyResult(msg applyResultMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.status = editorErr
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.status = editorDone
	return m, func() tea.Msg { return ClosedMsg{Applied: true} }
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// --- Discard confirmation overlay ---
	if m.confirmDiscard {
		switch msg.String() {
		case "ctrl+s":
			return m, func() tea.Msg { return ClosedMsg{Applied: false} }
		case "esc":
			m.confirmDiscard = false
			return m, nil
		}
		return m, nil // swallow all other keys
	}

	// --- Apply mode picker overlay ---
	if m.showApplyPicker {
		switch msg.String() {
		case "up", "k":
			if m.applyPickerIdx > 0 {
				m.applyPickerIdx--
			}
		case "down", "j":
			if m.applyPickerIdx < 2 {
				m.applyPickerIdx++
			}
		case "enter":
			return m.doApply()
		case "esc":
			m.showApplyPicker = false
		}
		return m, nil
	}

	// --- Normal editing ---
	switch msg.String() {
	case "ctrl+v":
		// Validate
		if m.status == editorValidating || m.status == editorApplying {
			return m, nil
		}
		m.status = editorValidating
		return m, m.cmdValidate()

	case "ctrl+s":
		// Apply: require valid (no errors) before showing picker
		switch m.status {
		case editorValidated:
			if len(m.validErrs) == 0 {
				m.showApplyPicker = true
				return m, nil
			}
			// There are validation errors — do nothing (user should fix first)
			return m, nil
		case editorIdle, editorErr:
			// Haven't validated yet: trigger validation then show picker if clean.
			// For UX simplicity: just validate first, picker appears after.
			m.status = editorValidating
			return m, m.cmdValidateThenPick()
		default:
			// Validating or applying in progress: ignore
			return m, nil
		}

	case "esc":
		if m.HasChanges() {
			m.confirmDiscard = true
			return m, nil
		}
		return m, func() tea.Msg { return ClosedMsg{Applied: false} }
	}

	// Delegate remaining keys to textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	// After editing, reset validated status so stale validation is cleared.
	if m.status == editorValidated {
		m.status = editorIdle
		m.validErrs = nil
		m.resize()
	}
	return m, cmd
}

// Timeouts for the config editor's backing gRPC calls. Validation is
// typically fast (server-side only); apply uploads the machine config and
// needs more headroom, especially on slower links.
const (
	configValidateTimeout = 30 * time.Second
	configApplyTimeout    = 60 * time.Second
)

func (m Model) doApply() (Model, tea.Cmd) {
	m.showApplyPicker = false
	m.status = editorApplying
	mode := applyModes[m.applyPickerIdx]
	yamlStr := m.textarea.Value()
	node := m.node
	client := m.client
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), configApplyTimeout)
		defer cancel()
		err := config.ApplyConfig(ctx, client, node, yamlStr, mode)
		return applyResultMsg{err: err}
	}
}

func (m Model) cmdValidate() tea.Cmd {
	yamlStr := m.textarea.Value()
	node := m.node
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), configValidateTimeout)
		defer cancel()
		errs, err := config.ValidateConfig(ctx, client, node, yamlStr)
		return validateResultMsg{errs: errs, err: err}
	}
}

// cmdValidateThenPick validates and, if clean, shows the apply picker.
// We accomplish this by using a wrapper that returns a special sentinel,
// then handleValidateResult opens the picker when appropriate.
func (m Model) cmdValidateThenPick() tea.Cmd {
	yamlStr := m.textarea.Value()
	node := m.node
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), configValidateTimeout)
		defer cancel()
		errs, err := config.ValidateConfig(ctx, client, node, yamlStr)
		return validateResultMsg{errs: errs, err: err, openPickerOnClean: true}
	}
}

// View renders the full-screen editor overlay.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	header := shared.StyleHeader.Render(fmt.Sprintf(" Editing config: %s", m.node))
	if m.status == editorApplying {
		header = shared.StyleWarning.Render(fmt.Sprintf(" Applying config: %s …", m.node))
	}
	sb.WriteString(header)
	sb.WriteString("\n")

	// Apply picker overlay (rendered in place of textarea)
	if m.showApplyPicker {
		sb.WriteString(m.renderApplyPicker())
	} else if m.status == editorErr {
		// Error state: show error message
		sb.WriteString(shared.StyleError.Render(" Error: " + m.errMsg))
		sb.WriteString("\n")
	} else {
		// Textarea
		sb.WriteString(m.textarea.View())
		sb.WriteString("\n")

		// Error panel (validation results)
		if len(m.validErrs) > 0 {
			sb.WriteString(m.renderErrorPanel())
		} else if m.status == editorValidated {
			sb.WriteString(shared.StyleSuccess.Render(" \u2713 Config is valid"))
			sb.WriteString("\n")
		}
	}

	// Discard confirmation
	if m.confirmDiscard {
		sb.WriteString(shared.StyleWarning.Render(" Discard unsaved changes? Ctrl+S=yes  Esc=cancel"))
		sb.WriteString("\n")
	} else {
		// Hints footer
		sb.WriteString(shared.StyleMuted.Render(" " + m.Hints()))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m Model) renderErrorPanel() string {
	const maxDisplay = 5
	var sb strings.Builder
	shown := m.validErrs
	extra := 0
	if len(shown) > maxDisplay {
		extra = len(shown) - maxDisplay
		shown = shown[:maxDisplay]
	}
	for _, e := range shown {
		sb.WriteString(shared.StyleError.Render(" \u2717 " + e))
		sb.WriteString("\n")
	}
	if extra > 0 {
		sb.WriteString(shared.StyleMuted.Render(fmt.Sprintf("   … %d more errors", extra)))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) renderApplyPicker() string {
	var sb strings.Builder
	sb.WriteString(shared.StyleHeader.Render(" Apply mode:"))
	sb.WriteString("\n")
	for i, label := range applyModeLabels {
		if i == m.applyPickerIdx {
			sb.WriteString(shared.StyleSelected.Render(fmt.Sprintf("  > %s", label)))
		} else {
			sb.WriteString(fmt.Sprintf("    %s", label))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(shared.StyleMuted.Render(" Enter=confirm  Esc=cancel"))
	sb.WriteString("\n")
	return sb.String()
}
