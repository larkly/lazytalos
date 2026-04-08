package help

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

type helpTier int

const (
	tierClosed   helpTier = 0
	tierProfiled helpTier = 1
	tierFull     helpTier = 2
)

// Model is the help overlay.
type Model struct {
	tier          helpTier
	Visible       bool
	view          string // current view name for profiled mode
	lines         []string
	scrollY       int
	width, height int
}

// New returns a new (hidden) help overlay.
func New() Model { return Model{} }

// Open cycles the help tier: closed → profiled → full → closed.
func (m *Model) Open(view string) {
	m.view = view
	switch m.tier {
	case tierClosed:
		m.tier = tierProfiled
	case tierProfiled:
		m.tier = tierFull
	default:
		m.tier = tierClosed
	}
	m.scrollY = 0
	m.Visible = m.tier != tierClosed
	if m.Visible {
		m.buildLines()
	}
}

// IsVisible reports whether the overlay is shown.
func (m Model) IsVisible() bool { return m.Visible }

// SetSize updates the terminal dimensions.
func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

// Update handles key input while the overlay is visible.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, shared.Keys.Help):
		if m.tier == tierProfiled {
			m.tier = tierFull
			m.scrollY = 0
			m.buildLines()
		} else {
			m.tier = tierClosed
			m.scrollY = 0
			m.Visible = false
		}
		return m, nil
	case key.Matches(keyMsg, shared.Keys.Up):
		if m.scrollY > 0 {
			m.scrollY--
		}
	case key.Matches(keyMsg, shared.Keys.Down):
		m.scrollY++
	case key.Matches(keyMsg, shared.Keys.Back):
		m.tier = tierClosed
		m.scrollY = 0
		m.Visible = false
	default:
		m.tier = tierClosed
		m.scrollY = 0
		m.Visible = false
	}
	return m, nil
}

// View renders the help overlay centred on screen.
func (m Model) View() string {
	maxW := 80
	if m.width < maxW {
		maxW = m.width
	}
	innerW := maxW - 4
	maxH := int(float64(m.height) * 0.9)
	visibleLines := maxH - 4

	if visibleLines < 1 {
		visibleLines = 1
	}

	// Clamp scroll
	if m.scrollY > len(m.lines)-visibleLines {
		m.scrollY = len(m.lines) - visibleLines
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}

	end := m.scrollY + visibleLines
	if end > len(m.lines) {
		end = len(m.lines)
	}
	visible := m.lines[m.scrollY:end]

	body := strings.Join(visible, "\n")

	title := shared.StyleHeader.Render("LAZYTALOS — Keyboard Reference")

	var hint string
	scrollHint := ""
	if len(m.lines) > visibleLines {
		scrollHint = shared.StyleMuted.Render("↑↓ scroll  ")
	}
	if m.tier == tierProfiled {
		hint = scrollHint + shared.StyleMuted.Render("? all shortcuts • esc close")
	} else {
		hint = scrollHint + shared.StyleMuted.Render("? or esc to close")
	}

	content := fmt.Sprintf("%s\n%s\n%s", title, body, hint)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		shared.StyleModal.Width(innerW).Render(content))
}

// --- Help content sections ---

type helpSection struct {
	title string
	items []helpItem
}

type helpItem struct {
	key  string
	desc string
}

var allSections = []helpSection{
	{title: "GLOBAL", items: []helpItem{
		{"q / ctrl+c", "Quit"},
		{"?", "Toggle help (? context, ?? full)"},
		{"C", "Switch cluster context"},
		{"Ctrl+,", "View talosconfig"},
		{"1–8", "Switch to tab by number"},
		{"← / →", "Previous / next tab"},
		{"Ctrl+R", "Force refresh"},
		{"↑ / ↓", "Navigate list"},
		{"PgUp / PgDn", "Scroll page"},
		{"Enter", "Detail view / confirm"},
		{"Esc", "Back / close"},
	}},
	{title: "DASHBOARD", items: []helpItem{
		{"↑ / ↓", "Navigate nodes"},
		{"e", "Toggle events follow"},
	}},
	{title: "NODES", items: []helpItem{
		{"Space", "Select / deselect node"},
		{"A", "Select all / deselect all"},
		{"Ctrl+O", "Reboot selected nodes"},
		{"Ctrl+D", "Shutdown selected nodes"},
		{"Ctrl+U", "Upgrade cluster (rolling)"},
		{"Ctrl+X", "Reset node (dangerous)"},
		{"Ctrl+E", "Edit node config"},
		{"y", "Copy node IP to clipboard"},
		{"Y", "Copy endpoint to clipboard"},
		{"/", "Filter"},
		{"s", "Cycle sort column"},
	}},
	{title: "SERVICES", items: []helpItem{
		{"Ctrl+K", "Restart service"},
		{"/", "Filter"},
		{"s", "Cycle sort column"},
		{"g", "Group by node"},
	}},
	{title: "LOGS", items: []helpItem{
		{"Tab", "Switch selector column"},
		{"Space / Enter", "Toggle node / service"},
		{"F", "Toggle follow mode"},
		{"Esc", "Focus log pane"},
	}},
	{title: "CONTAINERS", items: []helpItem{
		{"/", "Filter"},
		{"s", "Cycle sort column"},
		{"n", "Cycle node filter"},
		{"Ctrl+L", "View container logs"},
	}},
	{title: "NETWORK", items: []helpItem{
		{"< / >", "Previous / next sub-tab"},
		{"↑ / ↓", "Scroll"},
		{"Ctrl+R", "Force refresh"},
	}},
	{title: "STORAGE", items: []helpItem{
		{"< / >", "Previous / next sub-tab"},
		{"↑ / ↓", "Navigate"},
		{"Ctrl+R", "Force refresh"},
	}},
	{title: "ETCD", items: []helpItem{
		{"< / >", "Previous / next sub-tab"},
		{"Ctrl+M", "Remove etcd member"},
	}},
	{title: "UPGRADE", items: []helpItem{
		{"Ctrl+P", "Pause after current node"},
		{"Ctrl+A", "Abort upgrade"},
	}},
}

// viewSections maps view names to which section titles appear in profiled mode.
var viewSections = map[string][]string{
	"dashboard":  {"DASHBOARD"},
	"nodes":      {"NODES"},
	"services":   {"SERVICES"},
	"logs":       {"LOGS"},
	"containers": {"CONTAINERS"},
	"network":    {"NETWORK"},
	"storage":    {"STORAGE"},
	"etcd":       {"ETCD"},
}

func (m *Model) buildLines() {
	var lines []string
	add := func(s string) { lines = append(lines, s) }
	section := func(title string) { add(""); add(shared.StyleLabel.Render(title)) }
	item := func(k, desc string) {
		add(fmt.Sprintf("  %-18s %s", shared.StyleValue.Render(k), shared.StyleMuted.Render(desc)))
	}

	// Global section always shown
	for _, s := range allSections {
		if s.title == "GLOBAL" {
			section(s.title)
			for _, it := range s.items {
				item(it.key, it.desc)
			}
			break
		}
	}

	if m.tier == tierProfiled {
		// Show only sections relevant to the current view
		wanted := viewSections[m.view]
		for _, s := range allSections {
			for _, w := range wanted {
				if s.title == w {
					section(s.title)
					for _, it := range s.items {
						item(it.key, it.desc)
					}
				}
			}
		}
	} else {
		// Full mode: show all sections (skip GLOBAL, already added)
		for _, s := range allSections {
			if s.title == "GLOBAL" {
				continue
			}
			section(s.title)
			for _, it := range s.items {
				item(it.key, it.desc)
			}
		}
	}

	m.lines = lines
}
