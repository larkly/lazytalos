// Package configeditor provides an inline YAML editor for per-node machine configuration.
package configeditor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// Apply mode constants matching the PRD.
const (
	ModeNoReboot = 0
	ModeReboot   = 1
	ModeStaged   = 2
)

var modeLabels = []string{"No Reboot", "Reboot", "Staged"}

// Internal messages.
type configFetchedMsg struct {
	data []byte
	err  error
}

type configValidateResultMsg struct {
	errors []string
	err    error
}

type configApplyResultMsg struct {
	err error
}

// Model is the full-screen config editor model.
type Model struct {
	client *talos.Client
	node   string

	// Editor content
	lines     []string
	cursorRow int
	cursorCol int
	scrollY   int
	modified  bool

	// Validation
	validationErrors []string
	showValidation   bool

	// Apply mode selector
	showModeSelector bool
	selectedMode     int

	// State
	loading  bool
	applying bool
	err      error
	width    int
	height   int
}

// New creates a new config editor for the given node.
func New(client *talos.Client, node string) Model {
	return Model{
		client:  client,
		node:    node,
		loading: true,
	}
}

// Init returns the initial command to fetch the config.
func (m Model) Init() tea.Cmd {
	return m.fetchConfig()
}

func (m Model) fetchConfig() tea.Cmd {
	client := m.client
	node := m.node
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		nodeCtx := talosclient.WithNodes(ctx, node)
		meta := resource.NewMetadata(
			configres.NamespaceName,
			configres.MachineConfigType,
			configres.ActiveID,
			resource.VersionUndefined,
		)
		res, err := client.C.COSI.Get(nodeCtx, meta)
		if err != nil {
			return configFetchedMsg{err: fmt.Errorf("fetch config: %w", err)}
		}
		mc, ok := res.(*configres.MachineConfig)
		if !ok {
			return configFetchedMsg{err: fmt.Errorf("unexpected resource type: %T", res)}
		}
		yamlBytes, err := mc.Provider().Bytes()
		if err != nil {
			return configFetchedMsg{err: fmt.Errorf("encode config: %w", err)}
		}
		return configFetchedMsg{data: yamlBytes}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configFetchedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.loading = false
			return m, nil
		}
		m.lines = strings.Split(string(msg.data), "\n")
		m.loading = false
		m.err = nil
		return m, nil

	case configValidateResultMsg:
		m.applying = false
		if msg.err != nil {
			m.validationErrors = []string{msg.err.Error()}
		} else {
			m.validationErrors = msg.errors
		}
		m.showValidation = true
		return m, nil

	case configApplyResultMsg:
		m.applying = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Success — emit apply message to app
		return m, func() tea.Msg {
			return shared.ConfigAppliedMsg{Node: m.node}
		}

	case tea.KeyMsg:
		if m.showModeSelector {
			return m.updateModeSelector(msg)
		}
		if m.showValidation {
			return m.updateValidation(msg)
		}
		return m.updateEditor(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) updateEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Back):
		// Exit editor — app will handle restoring previous view
		return m, func() tea.Msg {
			return shared.ViewChangeMsg{}
		}

	// Validate: ctrl+v
	case msg.String() == "ctrl+v":
		m.applying = true
		return m, m.validateConfig()

	// Apply: ctrl+s
	case key.Matches(msg, shared.Keys.Confirm):
		m.showModeSelector = true
		m.selectedMode = ModeNoReboot
		return m, nil

	// Cursor movement
	case key.Matches(msg, shared.Keys.Up):
		if m.cursorRow > 0 {
			m.cursorRow--
			m.clampCol()
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.Down):
		if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
			m.clampCol()
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.Left):
		if m.cursorCol > 0 {
			m.cursorCol--
		} else if m.cursorRow > 0 {
			m.cursorRow--
			m.cursorCol = len(m.currentLine())
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.Right):
		if m.cursorCol < len(m.currentLine()) {
			m.cursorCol++
		} else if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
			m.cursorCol = 0
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.PageUp):
		m.cursorRow -= m.editorHeight()
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.clampCol()
		m.adjustScroll()
	case key.Matches(msg, shared.Keys.PageDown):
		m.cursorRow += m.editorHeight()
		if m.cursorRow >= len(m.lines) {
			m.cursorRow = len(m.lines) - 1
		}
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.clampCol()
		m.adjustScroll()

	// Home/End
	case msg.String() == "home":
		m.cursorCol = 0
	case msg.String() == "end":
		m.cursorCol = len(m.currentLine())

	// Text editing
	case msg.String() == "backspace":
		if m.cursorCol > 0 {
			line := m.currentLine()
			m.lines[m.cursorRow] = line[:m.cursorCol-1] + line[m.cursorCol:]
			m.cursorCol--
			m.modified = true
		} else if m.cursorRow > 0 {
			// Join with previous line
			prevLine := m.lines[m.cursorRow-1]
			m.cursorCol = len(prevLine)
			m.lines[m.cursorRow-1] = prevLine + m.lines[m.cursorRow]
			m.lines = append(m.lines[:m.cursorRow], m.lines[m.cursorRow+1:]...)
			m.cursorRow--
			m.modified = true
			m.adjustScroll()
		}
	case msg.String() == "delete":
		line := m.currentLine()
		if m.cursorCol < len(line) {
			m.lines[m.cursorRow] = line[:m.cursorCol] + line[m.cursorCol+1:]
			m.modified = true
		} else if m.cursorRow < len(m.lines)-1 {
			// Join with next line
			m.lines[m.cursorRow] = line + m.lines[m.cursorRow+1]
			m.lines = append(m.lines[:m.cursorRow+1], m.lines[m.cursorRow+2:]...)
			m.modified = true
		}
	case msg.String() == "enter":
		line := m.currentLine()
		before := line[:m.cursorCol]
		after := line[m.cursorCol:]
		m.lines[m.cursorRow] = before
		// Insert new line after current
		newLines := make([]string, len(m.lines)+1)
		copy(newLines, m.lines[:m.cursorRow+1])
		newLines[m.cursorRow+1] = after
		copy(newLines[m.cursorRow+2:], m.lines[m.cursorRow+1:])
		m.lines = newLines
		m.cursorRow++
		m.cursorCol = 0
		m.modified = true
		m.adjustScroll()
	case msg.String() == "tab":
		// Insert two spaces for YAML indent
		line := m.currentLine()
		m.lines[m.cursorRow] = line[:m.cursorCol] + "  " + line[m.cursorCol:]
		m.cursorCol += 2
		m.modified = true

	default:
		// Regular character input
		s := msg.String()
		if len(s) == 1 && s[0] >= 32 && s[0] <= 126 {
			line := m.currentLine()
			m.lines[m.cursorRow] = line[:m.cursorCol] + s + line[m.cursorCol:]
			m.cursorCol++
			m.modified = true
		}
	}
	return m, nil
}

func (m Model) updateModeSelector(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Back):
		m.showModeSelector = false
	case key.Matches(msg, shared.Keys.Up):
		if m.selectedMode > 0 {
			m.selectedMode--
		}
	case key.Matches(msg, shared.Keys.Down):
		if m.selectedMode < len(modeLabels)-1 {
			m.selectedMode++
		}
	case key.Matches(msg, shared.Keys.Enter):
		m.showModeSelector = false
		m.applying = true
		return m, m.applyConfig()
	}
	return m, nil
}

func (m Model) updateValidation(msg tea.KeyMsg) (Model, tea.Cmd) {
	if key.Matches(msg, shared.Keys.Back) || key.Matches(msg, shared.Keys.Enter) {
		m.showValidation = false
	}
	return m, nil
}

func (m Model) currentLine() string {
	if m.cursorRow < 0 || m.cursorRow >= len(m.lines) {
		return ""
	}
	return m.lines[m.cursorRow]
}

func (m *Model) clampCol() {
	lineLen := len(m.currentLine())
	if m.cursorCol > lineLen {
		m.cursorCol = lineLen
	}
}

func (m Model) editorHeight() int {
	h := m.height - 3 // status line + header + bottom border
	if m.showValidation {
		h -= 6 // validation panel
	}
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) adjustScroll() {
	visible := m.editorHeight()
	if m.cursorRow < m.scrollY {
		m.scrollY = m.cursorRow
	}
	if m.cursorRow >= m.scrollY+visible {
		m.scrollY = m.cursorRow - visible + 1
	}
}

func (m Model) configBytes() []byte {
	return []byte(strings.Join(m.lines, "\n"))
}

func (m Model) validateConfig() tea.Cmd {
	client := m.client
	node := m.node
	data := m.configBytes()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		nodeCtx := talosclient.WithNodes(ctx, node)
		resp, err := client.C.ApplyConfiguration(nodeCtx, &machine.ApplyConfigurationRequest{
			Data:   data,
			DryRun: true,
			Mode:   machine.ApplyConfigurationRequest_AUTO,
		})
		if err != nil {
			return configValidateResultMsg{err: err}
		}

		var errs []string
		for _, msg := range resp.GetMessages() {
			errs = append(errs, msg.GetWarnings()...)
			if details := msg.GetModeDetails(); details != "" {
				errs = append(errs, details)
			}
		}
		if len(errs) == 0 {
			errs = []string{"Validation passed - no errors found"}
		}
		return configValidateResultMsg{errors: errs}
	}
}

func (m Model) applyConfig() tea.Cmd {
	client := m.client
	node := m.node
	data := m.configBytes()
	mode := m.selectedMode
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		nodeCtx := talosclient.WithNodes(ctx, node)
		var grpcMode machine.ApplyConfigurationRequest_Mode
		switch mode {
		case ModeNoReboot:
			grpcMode = machine.ApplyConfigurationRequest_NO_REBOOT
		case ModeReboot:
			grpcMode = machine.ApplyConfigurationRequest_REBOOT
		case ModeStaged:
			grpcMode = machine.ApplyConfigurationRequest_STAGED
		}

		_, err := client.C.ApplyConfiguration(nodeCtx, &machine.ApplyConfigurationRequest{
			Data: data,
			Mode: grpcMode,
		})
		return configApplyResultMsg{err: err}
	}
}

// View renders the config editor.
func (m Model) View() string {
	if m.loading {
		msg := shared.StyleModalTitle.Render(fmt.Sprintf("Loading config for %s...", m.node))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}
	if m.err != nil && len(m.lines) == 0 {
		msg := shared.StyleError.Render(fmt.Sprintf("Error: %v", m.err))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}

	var b strings.Builder

	// Header bar
	modifiedMarker := ""
	if m.modified {
		modifiedMarker = " [modified]"
	}
	applyingMarker := ""
	if m.applying {
		applyingMarker = " (applying...)"
	}
	headerText := fmt.Sprintf(" Config: %s%s%s  |  ctrl+v:validate  ctrl+s:apply  esc:close",
		m.node, modifiedMarker, applyingMarker)
	header := lipgloss.NewStyle().
		Background(shared.ColorPrimary).
		Foreground(shared.ColorBg).
		Bold(true).
		Width(m.width).
		Render(headerText)
	b.WriteString(header)
	b.WriteString("\n")

	// Editor area
	visible := m.editorHeight()
	lineNumWidth := len(fmt.Sprintf("%d", len(m.lines)))
	if lineNumWidth < 3 {
		lineNumWidth = 3
	}
	contentWidth := m.width - lineNumWidth - 2 // separator

	end := m.scrollY + visible
	if end > len(m.lines) {
		end = len(m.lines)
	}

	lineNumStyle := lipgloss.NewStyle().Foreground(shared.ColorMuted)
	cursorLineStyle := lipgloss.NewStyle().Background(lipgloss.Color("#073642"))

	for i := m.scrollY; i < end; i++ {
		lineNum := lineNumStyle.Render(fmt.Sprintf("%*d", lineNumWidth, i+1))
		line := m.lines[i]

		// Colorize YAML
		coloredLine := m.colorizeYAML(line, contentWidth)

		if i == m.cursorRow {
			// Render cursor line with highlight
			rendered := fmt.Sprintf("%s %s", lineNum, coloredLine)
			b.WriteString(cursorLineStyle.Width(m.width).Render(rendered))
		} else {
			b.WriteString(fmt.Sprintf("%s %s", lineNum, coloredLine))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad remaining space
	rendered := end - m.scrollY
	for i := rendered; i < visible; i++ {
		b.WriteString("\n" + lineNumStyle.Render(strings.Repeat(" ", lineNumWidth)) + " ~")
	}

	// Mode selector overlay
	if m.showModeSelector {
		content := b.String()
		overlay := m.renderModeSelector()
		return content + "\n" + overlay
	}

	// Validation panel
	if m.showValidation {
		b.WriteString("\n")
		b.WriteString(m.renderValidation())
	}

	return b.String()
}

func (m Model) colorizeYAML(line string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	// Truncate long lines
	if len(line) > maxWidth {
		line = line[:maxWidth-3] + "..."
	}

	// Comment
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return lipgloss.NewStyle().Foreground(shared.ColorMuted).Render(line)
	}

	// Key: value
	if idx := strings.Index(line, ":"); idx > 0 {
		keyPart := line[:idx+1]
		valuePart := ""
		if idx+1 < len(line) {
			valuePart = line[idx+1:]
		}
		colored := lipgloss.NewStyle().Foreground(shared.ColorPrimary).Render(keyPart) + valuePart
		return colored
	}

	// List item
	if strings.HasPrefix(trimmed, "- ") {
		return lipgloss.NewStyle().Foreground(shared.ColorSecondary).Render(line)
	}

	return line
}

func (m Model) renderModeSelector() string {
	title := shared.StyleModalTitle.Render("Apply Mode")
	var items []string
	for i, label := range modeLabels {
		prefix := "  "
		if i == m.selectedMode {
			prefix = "▸ "
			items = append(items, shared.StyleSelected.Render(prefix+label))
		} else {
			items = append(items, prefix+label)
		}
	}
	hint := shared.StyleMuted.Render("\n↑↓:select  enter:apply  esc:cancel")
	content := title + "\n" + strings.Join(items, "\n") + hint
	box := shared.StyleModal.Width(40).Render(content)
	return lipgloss.Place(m.width, m.height/3, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderValidation() string {
	title := "Validation Results"
	style := shared.StyleSuccess
	for _, e := range m.validationErrors {
		if !strings.Contains(e, "passed") {
			style = shared.StyleError
			break
		}
	}
	titleRendered := style.Bold(true).Render(title)

	var lines []string
	maxShow := 5
	for i, e := range m.validationErrors {
		if i >= maxShow {
			lines = append(lines, shared.StyleMuted.Render(
				fmt.Sprintf("  ... and %d more", len(m.validationErrors)-maxShow)))
			break
		}
		lines = append(lines, "  "+e)
	}
	hint := shared.StyleMuted.Render("  press esc to dismiss")
	return titleRendered + "\n" + strings.Join(lines, "\n") + "\n" + hint
}

// SetSize updates the editor dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// ForceRefresh re-fetches the config.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchConfig()
}

// Hints returns status bar hint text.
func (m Model) Hints() string {
	return "ctrl+v:validate  ctrl+s:apply  esc:close"
}

// Node returns the node being edited.
func (m Model) Node() string {
	return m.node
}
