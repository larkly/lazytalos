// Package containers provides the Containers tab view with namespace filtering and detail.
package containers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
)

// containerRow is a flattened representation of a single container.
type containerRow struct {
	Node      string
	Namespace string
	Name      string
	ID        string
	Image     string
	Status    string
	PID       uint32
	PodID     string
}

// containersLoadedMsg is sent when container data has been fetched.
type containersLoadedMsg struct {
	containers []containerRow
	err        error
}

// Model is the containers tab view model.
type Model struct {
	client          *talos.Client
	containers      []containerRow
	filtered        []containerRow
	cursor          int
	scrollOff       int
	filter          string
	filterActive    bool
	namespace       string // "all", "k8s.io", "system"
	detailView      bool
	loading         bool
	err             error
	width           int
	height          int
	refreshInterval time.Duration
}

// New creates a new containers model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		namespace:       "all",
		loading:         true,
		refreshInterval: refreshInterval,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return m.ForceRefresh()
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filterActive {
			return m.updateFilter(msg)
		}
		if m.detailView {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)

	case shared.TickMsg:
		return m, m.fetchContainers()

	case containersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.containers = msg.containers
			m.applyFilter()
		}
		m.loading = false
	}

	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Down):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.PageDown):
		m.cursor += m.visibleRows()
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.adjustScroll()
	case key.Matches(msg, shared.Keys.PageUp):
		m.cursor -= m.visibleRows()
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.adjustScroll()
	case key.Matches(msg, shared.Keys.Enter):
		if m.cursor < len(m.filtered) {
			m.detailView = true
		}
	case key.Matches(msg, shared.Keys.Filter):
		m.filterActive = true
	case key.Matches(msg, shared.Keys.NamespaceToggle):
		m.cycleNamespace()
		m.applyFilter()
		return m, m.fetchContainers()
	case key.Matches(msg, shared.Keys.Back):
		if m.filter != "" {
			m.filter = ""
			m.applyFilter()
		} else {
			return m, func() tea.Msg { return shared.ViewChangeMsg{} }
		}
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Back):
		m.filterActive = false
	case key.Matches(msg, shared.Keys.Enter):
		m.filterActive = false
	default:
		s := msg.String()
		if s == "backspace" {
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			}
		} else if len(s) == 1 {
			m.filter += s
		}
	}
	m.applyFilter()
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (Model, tea.Cmd) {
	if key.Matches(msg, shared.Keys.Back) {
		m.detailView = false
	}
	return m, nil
}

func (m *Model) cycleNamespace() {
	switch m.namespace {
	case "all":
		m.namespace = "k8s.io"
	case "k8s.io":
		m.namespace = "system"
	case "system":
		m.namespace = "all"
	}
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.containers
	} else {
		lower := strings.ToLower(m.filter)
		m.filtered = nil
		for _, c := range m.containers {
			if strings.Contains(strings.ToLower(c.Name), lower) ||
				strings.Contains(strings.ToLower(c.Node), lower) ||
				strings.Contains(strings.ToLower(c.Namespace), lower) ||
				strings.Contains(strings.ToLower(c.Image), lower) {
				m.filtered = append(m.filtered, c)
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) adjustScroll() {
	visible := m.visibleRows()
	if m.cursor < m.scrollOff {
		m.scrollOff = m.cursor
	}
	if m.cursor >= m.scrollOff+visible {
		m.scrollOff = m.cursor - visible + 1
	}
}

func (m Model) visibleRows() int {
	v := m.height - 3 // header + column row + filter row
	if v < 1 {
		return 1
	}
	return v
}

// View renders the containers tab.
func (m Model) View() string {
	if m.detailView && m.cursor < len(m.filtered) {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m Model) viewList() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.loading && len(m.containers) == 0 {
		return shared.StyleMuted.Render("  Loading containers...")
	}

	var lines []string

	// Namespace indicator
	lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Namespace: %s", m.namespace)))

	// Filter row
	if m.filterActive {
		lines = append(lines, shared.StyleWarning.Render(fmt.Sprintf("  Filter: %s_", m.filter)))
	} else if m.filter != "" {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Filter: %s", m.filter)))
	}

	// Column header
	header := fmt.Sprintf("  %-16s %-8s %-24s %-36s %-10s %6s",
		"NODE", "NS", "NAME", "IMAGE", "STATUS", "PID")
	lines = append(lines, shared.StyleHeader.Render(header))

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	visible := m.visibleRows()
	endIdx := m.scrollOff + visible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	for i := m.scrollOff; i < endIdx; i++ {
		c := m.filtered[i]

		cursor := " "
		isCursor := false
		if i == m.cursor {
			cursor = ">"
			isCursor = true
		}

		statusIcon := shared.StatusIcon(c.Status)
		statusStyle := shared.StyleSuccess
		if c.Status != "RUNNING" {
			statusIcon = shared.StatusIcon("Stopped")
			statusStyle = shared.StyleWarning
		}

		ns := c.Namespace
		if len(ns) > 8 {
			ns = ns[:6] + ".."
		}

		row := fmt.Sprintf("%s %-16s %-8s %-24s %-36s %s%-9s %6d",
			cursor,
			truncate(c.Node, 16),
			ns,
			truncate(c.Name, 24),
			truncate(c.Image, 36),
			statusStyle.Render(statusIcon),
			c.Status,
			c.PID,
		)
		lines = append(lines, renderRow(isCursor, row))
	}

	return strings.Join(lines, "\n")
}

func (m Model) viewDetail() string {
	c := m.filtered[m.cursor]
	var lines []string

	lines = append(lines, shared.StyleHeader.Render(fmt.Sprintf("  Container: %s", c.Name)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Node:"), c.Node))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Namespace:"), c.Namespace))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Name:"), c.Name))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("ID:"), c.ID))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Image:"), c.Image))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Status:"), c.Status))
	lines = append(lines, fmt.Sprintf("  %-20s %d", shared.StyleLabel.Render("PID:"), c.PID))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Pod ID:"), c.PodID))
	lines = append(lines, "")
	lines = append(lines, shared.StyleMuted.Render("  Press Esc to go back"))

	return strings.Join(lines, "\n")
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Hints returns status bar hint text.
func (m Model) Hints() string {
	if m.detailView {
		return "esc:back"
	}
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	return "enter:detail  /:filter  n:namespace"
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchContainers()
}

func (m Model) fetchContainers() tea.Cmd {
	client := m.client
	ns := m.namespace
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return containersLoadedMsg{err: fmt.Errorf("no client connection")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var rows []containerRow

		switch ns {
		case "k8s.io":
			r, err := fetchNamespace(ctx, client, "k8s.io", common.ContainerDriver_CRI)
			if err != nil {
				return containersLoadedMsg{err: err}
			}
			rows = r
		case "system":
			r, err := fetchNamespace(ctx, client, "system", common.ContainerDriver_CONTAINERD)
			if err != nil {
				return containersLoadedMsg{err: err}
			}
			rows = r
		default: // "all"
			r1, err := fetchNamespace(ctx, client, "k8s.io", common.ContainerDriver_CRI)
			if err != nil {
				return containersLoadedMsg{err: err}
			}
			r2, err := fetchNamespace(ctx, client, "system", common.ContainerDriver_CONTAINERD)
			if err != nil {
				return containersLoadedMsg{err: err}
			}
			rows = append(r1, r2...)
		}

		return containersLoadedMsg{containers: rows}
	}
}

func fetchNamespace(ctx context.Context, client *talos.Client, namespace string, driver common.ContainerDriver) ([]containerRow, error) {
	resp, err := client.C.Containers(ctx, namespace, driver)
	if err != nil {
		return nil, err
	}

	var rows []containerRow
	for _, msg := range resp.GetMessages() {
		hostname := ""
		if msg.GetMetadata() != nil {
			hostname = msg.GetMetadata().GetHostname()
		}
		for _, ci := range msg.GetContainers() {
			rows = append(rows, containerRow{
				Node:      hostname,
				Namespace: ci.GetNamespace(),
				Name:      ci.GetName(),
				ID:        ci.GetId(),
				Image:     ci.GetImage(),
				Status:    ci.GetStatus(),
				PID:       ci.GetPid(),
				PodID:     ci.GetPodId(),
			})
		}
	}
	return rows, nil
}

func renderRow(highlighted bool, s string) string {
	if highlighted {
		return shared.StyleSelected.Render(s)
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "\u2026"
}
