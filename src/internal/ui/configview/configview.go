package configview

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	talosclientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"

	"github.com/larkly/lazytalos/internal/shared"
)

// ContextEntry holds display info for one talosconfig context.
type ContextEntry struct {
	Name      string
	CADigest  string
	Endpoints []string
}

// ClosedMsg is emitted when the overlay is dismissed.
type ClosedMsg struct{}

// Model is the talosconfig view overlay.
type Model struct {
	visible    bool
	configPath string
	contexts   []ContextEntry
	activeCtx  string
	scrollY    int
	width      int
	height     int
	err        string
}

// New creates a new (hidden) configview overlay.
func New(configPath string) Model {
	return Model{configPath: configPath}
}

// IsVisible reports whether the overlay is shown.
func (m Model) IsVisible() bool { return m.visible }

// SetSize updates terminal dimensions.
func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

// SetActiveContext updates the active context name.
func (m *Model) SetActiveContext(name string) { m.activeCtx = name }

// Toggle flips visibility; when going visible, reloads config from disk.
func (m Model) Toggle() Model {
	m.visible = !m.visible
	m.scrollY = 0
	if m.visible {
		m = m.load()
	}
	return m
}

// load reads the talosconfig from disk and populates m.contexts.
func (m Model) load() Model {
	m.contexts = nil
	m.err = ""

	cfg, err := talosclientconfig.Open(m.configPath)
	if err != nil {
		m.err = err.Error()
		return m
	}

	// Preserve active context from config file if not set externally.
	if m.activeCtx == "" {
		m.activeCtx = cfg.Context
	}

	for name, ctx := range cfg.Contexts {
		entry := ContextEntry{
			Name:      name,
			Endpoints: ctx.Endpoints,
			CADigest:  caDigest(ctx.CA),
		}
		m.contexts = append(m.contexts, entry)
	}

	// Sort for stable display order
	sortContextEntries(m.contexts)

	return m
}

// caDigest returns the first 16 hex chars of the SHA-256 of the decoded CA PEM.
// Returns "" if CA is empty or cannot be decoded.
func caDigest(caBase64 string) string {
	if caBase64 == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(caBase64)
	if err != nil {
		// Try raw base64
		b, err = base64.RawStdEncoding.DecodeString(caBase64)
		if err != nil {
			return ""
		}
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])[:16]
}

func sortContextEntries(entries []ContextEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Name < entries[j-1].Name; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// Update handles key input while visible.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, shared.Keys.Up):
		if m.scrollY > 0 {
			m.scrollY--
		}
	case key.Matches(keyMsg, shared.Keys.Down):
		m.scrollY++
	case key.Matches(keyMsg, shared.Keys.Back):
		m.visible = false
		return m, func() tea.Msg { return ClosedMsg{} }
	default:
		m.visible = false
		return m, func() tea.Msg { return ClosedMsg{} }
	}
	return m, nil
}

// View renders the overlay centred on screen.
func (m Model) View() string {
	maxW := 70
	if m.width < maxW {
		maxW = m.width
	}
	innerW := maxW - 6 // borders + padding

	var sb strings.Builder

	title := shared.StyleHeader.Render("Talosconfig — Contexts")
	sb.WriteString(title)
	sb.WriteString("\n")

	if m.err != "" {
		sb.WriteString(shared.StyleError.Render("Error: " + m.err))
		sb.WriteString("\n")
	} else if len(m.contexts) == 0 {
		sb.WriteString(shared.StyleMuted.Render("No contexts found"))
		sb.WriteString("\n")
	} else {
		lines := m.buildLines()

		// Clamp scroll
		maxScroll := len(lines) - 1
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.scrollY > maxScroll {
			m.scrollY = maxScroll
		}

		maxVisible := int(float64(m.height)*0.85) - 6
		if maxVisible < 1 {
			maxVisible = 1
		}
		end := m.scrollY + maxVisible
		if end > len(lines) {
			end = len(lines)
		}
		for _, l := range lines[m.scrollY:end] {
			sb.WriteString(l)
			sb.WriteString("\n")
		}
	}

	footer := shared.StyleMuted.Render("Esc to close")
	sb.WriteString(footer)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		shared.StyleModal.Width(innerW).Render(sb.String()))
}

func (m Model) buildLines() []string {
	var lines []string
	for _, entry := range m.contexts {
		nameStyle := shared.StyleValue
		if entry.Name == m.activeCtx {
			nameStyle = shared.StyleSuccess
		}
		lines = append(lines, nameStyle.Render("● "+entry.Name))
		if len(entry.Endpoints) > 0 {
			lines = append(lines, shared.StyleMuted.Render("  endpoints:"))
			for _, ep := range entry.Endpoints {
				lines = append(lines, shared.StyleMuted.Render("    "+ep))
			}
		}
		if entry.CADigest != "" {
			lines = append(lines, shared.StyleMuted.Render("  ca: "+entry.CADigest+"…"))
		}
		lines = append(lines, "")
	}
	return lines
}
