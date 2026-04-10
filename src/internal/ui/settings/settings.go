package settings

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/larkly/lazytalos/internal/config"
	"github.com/larkly/lazytalos/internal/shared"
)

type itemKind int

const (
	kindBool itemKind = iota
	kindNumeric
	kindColor
	kindKeybinding
)

type configItem struct {
	label string
	key   string
	kind  itemKind
	get   func() string
	set   func(string) error
}

type category struct {
	name  string
	items []configItem
}

// Model is the settings overlay.
type Model struct {
	Visible bool
	Width   int
	Height  int

	cfg        *config.Config
	categories []category
	cursor     int
	editing    bool
	textInput  textinput.Model
	keyCapture bool
	errMsg     string
	scroll     int
}

// New creates a new settings overlay model.
func New(cfg *config.Config) Model {
	ti := textinput.New()
	ti.CharLimit = 32
	if cfg == nil {
		defaults := config.Defaults()
		cfg = &defaults
	}
	m := Model{
		cfg:       cfg,
		textInput: ti,
	}
	m.buildCategories()
	return m
}

// Cfg returns the config pointer.
func (m *Model) Cfg() *config.Config {
	return m.cfg
}

// Open shows the settings overlay.
func (m *Model) Open() {
	m.Visible = true
	m.cursor = 0
	m.editing = false
	m.keyCapture = false
	m.errMsg = ""
	m.scroll = 0
	m.buildCategories()
}

func (m *Model) buildCategories() {
	cfg := m.cfg
	m.categories = []category{
		{
			name: "General",
			items: []configItem{
				{label: "Refresh interval (s)", key: "refresh_interval", kind: kindNumeric,
					get: func() string { return strconv.Itoa(cfg.General.RefreshInterval) },
					set: func(v string) error {
						n, err := strconv.Atoi(v)
						if err != nil || n < 1 {
							return fmt.Errorf("must be a positive integer")
						}
						cfg.General.RefreshInterval = n
						return nil
					},
				},
				{label: "Plain mode", key: "plain_mode", kind: kindBool,
					get: func() string { return boolStr(cfg.General.PlainMode) },
					set: func(string) error {
						cfg.General.PlainMode = !cfg.General.PlainMode
						return nil
					},
				},
				{label: "Check for updates", key: "check_for_updates", kind: kindBool,
					get: func() string { return boolStr(cfg.General.CheckForUpdates) },
					set: func(string) error {
						cfg.General.CheckForUpdates = !cfg.General.CheckForUpdates
						return nil
					},
				},
				{label: "Update check interval (h)", key: "update_check_interval", kind: kindNumeric,
					get: func() string { return strconv.Itoa(cfg.General.UpdateCheckInterval) },
					set: func(v string) error {
						n, err := strconv.Atoi(v)
						if err != nil || n < 0 {
							return fmt.Errorf("must be a non-negative integer")
						}
						cfg.General.UpdateCheckInterval = n
						return nil
					},
				},
				{label: "Always pick context", key: "always_pick_context", kind: kindBool,
					get: func() string { return boolStr(cfg.General.AlwaysPickContext) },
					set: func(string) error {
						cfg.General.AlwaysPickContext = !cfg.General.AlwaysPickContext
						return nil
					},
				},
			},
		},
		{
			name: "Thresholds",
			items: []configItem{
				{label: "Memory warning (%)", key: "memory_warning", kind: kindNumeric,
					get: func() string { return strconv.Itoa(cfg.Thresholds.MemoryWarning) },
					set: func(v string) error {
						n, err := strconv.Atoi(v)
						if err != nil || n < 1 || n > 99 {
							return fmt.Errorf("must be 1-99")
						}
						cfg.Thresholds.MemoryWarning = n
						return nil
					},
				},
				{label: "Memory critical (%)", key: "memory_critical", kind: kindNumeric,
					get: func() string { return strconv.Itoa(cfg.Thresholds.MemoryCritical) },
					set: func(v string) error {
						n, err := strconv.Atoi(v)
						if err != nil || n < 1 || n > 99 {
							return fmt.Errorf("must be 1-99")
						}
						cfg.Thresholds.MemoryCritical = n
						return nil
					},
				},
				{label: "CPU warning (%)", key: "cpu_warning", kind: kindNumeric,
					get: func() string { return strconv.Itoa(cfg.Thresholds.CPUWarning) },
					set: func(v string) error {
						n, err := strconv.Atoi(v)
						if err != nil || n < 1 || n > 99 {
							return fmt.Errorf("must be 1-99")
						}
						cfg.Thresholds.CPUWarning = n
						return nil
					},
				},
			},
		},
		{
			name:  "Colors",
			items: m.buildColorItems(),
		},
		{
			name:  "Keybindings",
			items: m.buildKeybindingItems(),
		},
	}
}

func (m *Model) buildColorItems() []configItem {
	cfg := m.cfg
	type colorField struct {
		label string
		get   func() string
		set   func(string)
	}
	fields := []colorField{
		{"Primary", func() string { return cfg.Colors.Primary }, func(v string) { cfg.Colors.Primary = v }},
		{"Secondary", func() string { return cfg.Colors.Secondary }, func(v string) { cfg.Colors.Secondary = v }},
		{"Success", func() string { return cfg.Colors.Success }, func(v string) { cfg.Colors.Success = v }},
		{"Warning", func() string { return cfg.Colors.Warning }, func(v string) { cfg.Colors.Warning = v }},
		{"Error", func() string { return cfg.Colors.Error }, func(v string) { cfg.Colors.Error = v }},
		{"Muted", func() string { return cfg.Colors.Muted }, func(v string) { cfg.Colors.Muted = v }},
		{"Background", func() string { return cfg.Colors.Bg }, func(v string) { cfg.Colors.Bg = v }},
		{"Foreground", func() string { return cfg.Colors.Fg }, func(v string) { cfg.Colors.Fg = v }},
		{"Highlight", func() string { return cfg.Colors.Highlight }, func(v string) { cfg.Colors.Highlight = v }},
	}
	items := make([]configItem, len(fields))
	for i, f := range fields {
		f := f
		items[i] = configItem{
			label: f.label, kind: kindColor,
			get: f.get,
			set: func(v string) error {
				if !isValidHex(v) {
					return fmt.Errorf("invalid hex color (e.g. #FF0000)")
				}
				f.set(v)
				return nil
			},
		}
	}
	return items
}

func (m *Model) buildKeybindingItems() []configItem {
	cfg := m.cfg
	order := keybindingOrder()
	var items []configItem
	for _, name := range order {
		name := name
		if _, ok := cfg.Keybindings[name]; !ok {
			continue
		}
		items = append(items, configItem{
			label: keybindingLabel(name), key: name, kind: kindKeybinding,
			get: func() string { return cfg.Keybindings[name] },
			set: func(v string) error {
				cfg.Keybindings[name] = v
				return nil
			},
		})
	}
	return items
}

// Update handles messages for the settings overlay.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.keyCapture {
			return m.handleKeyCapture(msg)
		}
		if m.editing {
			return m.handleEditing(msg)
		}
		return m.handleBrowsing(msg)
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m Model) handleBrowsing(msg tea.KeyMsg) (Model, tea.Cmd) {
	m.errMsg = ""
	total := m.totalItems()

	switch {
	case key.Matches(msg, shared.Keys.Back):
		m.Visible = false
		return m, nil

	case key.Matches(msg, shared.Keys.Down):
		if m.cursor < total-1 {
			m.cursor++
			m.ensureVisible()
		}

	case key.Matches(msg, shared.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}

	case key.Matches(msg, shared.Keys.PageDown):
		m.cursor += m.viewHeight()
		if m.cursor >= total {
			m.cursor = total - 1
		}
		m.ensureVisible()

	case key.Matches(msg, shared.Keys.PageUp):
		m.cursor -= m.viewHeight()
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()

	case key.Matches(msg, shared.Keys.Enter), key.Matches(msg, shared.Keys.Select):
		item := m.currentItem()
		if item == nil {
			break
		}
		switch item.kind {
		case kindBool:
			if err := item.set(""); err != nil {
				m.errMsg = err.Error()
			} else {
				return m, m.applyAndSave()
			}
		case kindNumeric, kindColor:
			m.editing = true
			m.textInput.SetValue(item.get())
			m.textInput.Focus()
		case kindKeybinding:
			m.keyCapture = true
			m.errMsg = ""
		}
	}

	return m, nil
}

func (m Model) handleEditing(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Back):
		m.editing = false
		m.errMsg = ""
		return m, nil

	case key.Matches(msg, shared.Keys.Enter):
		item := m.currentItem()
		if item == nil {
			m.editing = false
			return m, nil
		}
		val := m.textInput.Value()
		if err := item.set(val); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.editing = false
		m.errMsg = ""
		return m, m.applyAndSave()

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m Model) handleKeyCapture(msg tea.KeyMsg) (Model, tea.Cmd) {
	keyStr := msg.String()

	if key.Matches(msg, shared.Keys.Back) {
		m.keyCapture = false
		m.errMsg = ""
		return m, nil
	}

	item := m.currentItem()
	if item == nil {
		m.keyCapture = false
		return m, nil
	}

	if err := item.set(keyStr); err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	m.keyCapture = false
	m.errMsg = ""
	return m, m.applyAndSave()
}

func (m *Model) applyAndSave() tea.Cmd {
	config.ApplyAll(*m.cfg)
	if err := m.cfg.Save(); err != nil {
		m.errMsg = "save failed: " + err.Error()
		shared.Debugf("[settings] save error: %s", err.Error())
	}
	return func() tea.Msg { return shared.ConfigChangedMsg{} }
}

func (m Model) currentItem() *configItem {
	idx := 0
	for ci := range m.categories {
		for ii := range m.categories[ci].items {
			if idx == m.cursor {
				return &m.categories[ci].items[ii]
			}
			idx++
		}
	}
	return nil
}

func (m Model) totalItems() int {
	n := 0
	for _, c := range m.categories {
		n += len(c.items)
	}
	return n
}

func (m Model) viewHeight() int {
	h := m.Height - 8
	if h < 5 {
		h = 5
	}
	return h
}

func (m *Model) ensureVisible() {
	line := m.cursorLine()
	vh := m.viewHeight()
	if line < m.scroll {
		m.scroll = line
	}
	if line >= m.scroll+vh {
		m.scroll = line - vh + 1
	}
}

func (m Model) cursorLine() int {
	line := 0
	idx := 0
	for _, c := range m.categories {
		line++ // category header
		for range c.items {
			if idx == m.cursor {
				return line
			}
			line++
			idx++
		}
		line++ // blank line after category
	}
	return line
}

// Render returns the settings overlay content.
func (m Model) Render() string {
	title := shared.StyleModalTitle.Render("Settings")

	lines := m.buildLines()

	vh := m.viewHeight()
	start := m.scroll
	if start > len(lines) {
		start = len(lines)
	}
	end := start + vh
	if end > len(lines) {
		end = len(lines)
	}

	visible := strings.Join(lines[start:end], "\n")

	errLine := ""
	if m.errMsg != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(shared.ColorError).Render("  "+m.errMsg)
	}

	scrollHint := ""
	if m.scroll > 0 || end < len(lines) {
		scrollHint = shared.StyleMuted.Render(" ↑↓ scroll •")
	}

	hint := scrollHint + shared.StyleMuted.Render(" esc close")
	if m.editing {
		hint = shared.StyleMuted.Render(" enter confirm • esc cancel")
	} else if m.keyCapture {
		hint = shared.StyleMuted.Render(" press key to bind • esc cancel")
	}

	content := title + "\n\n" + visible + errLine + "\n\n" + hint
	box := shared.StyleModal.Width(58).Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) buildLines() []string {
	var lines []string
	idx := 0
	labelStyle := lipgloss.NewStyle().Foreground(shared.ColorFg).Width(26)
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#073642")).Foreground(shared.ColorHighlight)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(shared.ColorSecondary)

	for _, c := range m.categories {
		lines = append(lines, headerStyle.Render("  "+c.name))
		for _, item := range c.items {
			selected := idx == m.cursor
			val := item.get()

			var line string
			switch item.kind {
			case kindBool:
				check := "[ ]"
				if val == "true" {
					check = "[x]"
				}
				line = "    " + labelStyle.Render(item.label) + check

			case kindNumeric:
				if selected && m.editing {
					line = "    " + labelStyle.Render(item.label) + m.textInput.View()
				} else {
					line = "    " + labelStyle.Render(item.label) + val
				}

			case kindColor:
				if selected && m.editing {
					line = "    " + labelStyle.Render(item.label) + m.textInput.View()
				} else {
					swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(val)).Render("████")
					line = "    " + labelStyle.Render(item.label) + val + "  " + swatch
				}

			case kindKeybinding:
				if selected && m.keyCapture {
					line = "    " + labelStyle.Render(item.label) +
						lipgloss.NewStyle().Foreground(shared.ColorWarning).Render("Press key...")
				} else {
					line = "    " + labelStyle.Render(item.label) + val
				}
			}

			if selected && !m.editing && !m.keyCapture {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, line)
			idx++
		}
		lines = append(lines, "")
	}
	return lines
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func isValidHex(s string) bool {
	return hexColorRe.MatchString(s)
}

func keybindingOrder() []string {
	return []string{
		"quit", "help", "settings", "context_picker",
		"filter", "enter", "back",
		"up", "down", "left", "right", "page_up", "page_down",
		"tab", "shift_tab",
		"select", "select_all", "sort", "refresh",
		"log_follow", "group_toggle",
		"reboot", "shutdown", "service_restart",
		"edit_config", "container_logs", "remove_etcd",
		"confirm", "upgrade_cluster", "reset_node",
		"pause_upgrade", "abort_upgrade",
		"yank_ip", "yank_endpoint", "config_view",
	}
}

func keybindingLabel(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
