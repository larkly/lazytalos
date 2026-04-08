// Package nodelist provides the cluster nodes tab view with space-select and detail.
package nodelist

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// Internal messages.
type membersLoadedMsg struct {
	nodes []cluster.NodeInfo
	err   error
}

type servicesLoadedMsg struct {
	servicesByNode map[string][]serviceInfo
	err            error
}

type serviceInfo struct {
	ID     string
	State  string
	Health string
}

// Model is the node list view model.
type Model struct {
	client          *talos.Client
	nodes           []cluster.NodeInfo
	filtered        []cluster.NodeInfo
	cursor          int
	selected        map[string]bool
	filter          string
	filterActive    bool
	detailView      bool
	detailServices  []serviceInfo
	loading         bool
	err             error
	width           int
	height          int
	scrollOff       int
	refreshInterval time.Duration
}

// New creates a new node list model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		selected:        make(map[string]bool),
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
		return m, m.fetchMembers()

	case membersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.nodes = msg.nodes
			m.applyFilter()
		}
		m.loading = false

	case servicesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else if m.detailView && m.cursor < len(m.filtered) {
			hostname := m.filtered[m.cursor].Hostname
			if svcs, ok := msg.servicesByNode[hostname]; ok {
				m.detailServices = svcs
			}
		}
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
	case key.Matches(msg, shared.Keys.Select):
		if m.cursor < len(m.filtered) {
			hostname := m.filtered[m.cursor].Hostname
			m.selected[hostname] = !m.selected[hostname]
			if !m.selected[hostname] {
				delete(m.selected, hostname)
			}
		}
	case key.Matches(msg, shared.Keys.SelectAll):
		if len(m.selected) == len(m.filtered) {
			m.selected = make(map[string]bool)
		} else {
			for _, n := range m.filtered {
				m.selected[n.Hostname] = true
			}
		}
	case key.Matches(msg, shared.Keys.Enter):
		if m.cursor < len(m.filtered) {
			m.detailView = true
			m.detailServices = nil
			return m, m.fetchServicesForDetail()
		}
	case key.Matches(msg, shared.Keys.Filter):
		m.filterActive = true
	case key.Matches(msg, shared.Keys.Reboot):
		return m, m.emitNodeAction("reboot")
	case key.Matches(msg, shared.Keys.Shutdown):
		return m, m.emitNodeAction("shutdown")
	case key.Matches(msg, shared.Keys.Back):
		if m.filter != "" {
			m.filter = ""
			m.applyFilter()
		} else {
			return m, func() tea.Msg { return shared.ViewChangeMsg{} }
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
	case key.Matches(msg, shared.Keys.YankIP):
		if m.cursor < len(m.filtered) && len(m.filtered[m.cursor].Addresses) > 0 {
			return m, func() tea.Msg {
				return shared.YankMsg{Text: m.filtered[m.cursor].Addresses[0]}
			}
		}
	case key.Matches(msg, shared.Keys.YankEndpoint):
		if m.client != nil && len(m.client.Endpoints) > 0 {
			return m, func() tea.Msg {
				return shared.YankMsg{Text: m.client.Endpoints[0]}
			}
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
	switch {
	case key.Matches(msg, shared.Keys.Back):
		m.detailView = false
	case key.Matches(msg, shared.Keys.ResetNode):
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			return m, func() tea.Msg {
				return shared.NodeResetRequestMsg{Node: m.filtered[m.cursor].Hostname}
			}
		}
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.nodes
	} else {
		lower := strings.ToLower(m.filter)
		m.filtered = nil
		for _, n := range m.nodes {
			if strings.Contains(strings.ToLower(n.Hostname), lower) ||
				strings.Contains(strings.ToLower(n.MachineType), lower) {
				m.filtered = append(m.filtered, n)
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

// View renders the node list.
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

	if m.loading && len(m.nodes) == 0 {
		return shared.StyleMuted.Render("  Loading nodes...")
	}

	var lines []string

	// Filter row
	if m.filterActive {
		lines = append(lines, shared.StyleWarning.Render(fmt.Sprintf("  Filter: %s_", m.filter)))
	} else if m.filter != "" {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Filter: %s", m.filter)))
	}

	// Column header
	header := fmt.Sprintf("  [ ] %-26s %-14s %-10s %-8s %-16s",
		"HOSTNAME", "TYPE", "VERSION", "HEALTH", "IP")
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
		n := m.filtered[i]
		check := "   "
		if m.selected[n.Hostname] {
			check = " x "
		}

		cursor := " "
		isCursor := false
		if i == m.cursor {
			cursor = ">"
			isCursor = true
		}

		typeStr := "worker"
		if n.IsControlPlane() {
			typeStr = "controlplane"
		}

		healthIcon := shared.StatusIcon("Running")
		healthStr := shared.StyleSuccess.Render(healthIcon)

		ip := ""
		if len(n.Addresses) > 0 {
			ip = n.Addresses[0]
		}

		row := fmt.Sprintf("%s[%s] %-26s %-14s %-10s %-8s %-16s",
			cursor,
			check,
			truncate(n.Hostname, 26),
			typeStr,
			n.TalosVersion,
			healthStr,
			ip,
		)
		lines = append(lines, renderRow(isCursor, row))
	}

	// Selection info
	if len(m.selected) > 0 {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  %d node(s) selected", len(m.selected))))
	}

	content := strings.Join(lines, "\n")
	return content
}

func (m Model) viewDetail() string {
	n := m.filtered[m.cursor]
	var lines []string

	lines = append(lines, shared.StyleHeader.Render(fmt.Sprintf("  Node: %s", n.Hostname)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Type:"), n.MachineType))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Talos Version:"), n.TalosVersion))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Healthy:"), fmt.Sprintf("%v", n.Healthy)))

	lines = append(lines, "")
	lines = append(lines, shared.StyleLabel.Render("  Addresses:"))
	for _, a := range n.Addresses {
		lines = append(lines, fmt.Sprintf("    %s", a))
	}

	lines = append(lines, "")
	lines = append(lines, shared.StyleLabel.Render("  Services:"))
	if len(m.detailServices) == 0 {
		lines = append(lines, shared.StyleMuted.Render("    Loading..."))
	} else {
		for _, s := range m.detailServices {
			icon := shared.StatusIcon("Running")
			style := shared.StyleSuccess
			if s.State != "Running" || s.Health == "Failed" {
				icon = shared.StatusIcon("Failed")
				style = shared.StyleError
			}
			lines = append(lines, fmt.Sprintf("    %s %-20s %-12s %s", style.Render(icon), s.ID, s.State, s.Health))
		}
	}

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
		return "esc:back  ctrl+x:reset"
	}
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	return "space:select  A:all  enter:detail  /:filter  y:copy IP  Y:copy endpoint  ctrl+o:reboot  ctrl+d:shutdown"
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchMembers()
}

// SelectedNodes returns the list of selected node hostnames, or the cursor node if none selected.
func (m Model) SelectedNodes() []cluster.NodeInfo {
	if len(m.selected) > 0 {
		var result []cluster.NodeInfo
		for _, n := range m.filtered {
			if m.selected[n.Hostname] {
				result = append(result, n)
			}
		}
		return result
	}
	if m.cursor < len(m.filtered) {
		return []cluster.NodeInfo{m.filtered[m.cursor]}
	}
	return nil
}

func (m Model) emitNodeAction(action string) tea.Cmd {
	nodes := m.SelectedNodes()
	if len(nodes) == 0 {
		return nil
	}
	hostnames := make([]string, len(nodes))
	names := make([]string, len(nodes))
	isCP := make([]bool, len(nodes))
	for i, n := range nodes {
		hostnames[i] = n.Hostname
		names[i] = n.Hostname
		isCP[i] = n.IsControlPlane()
	}
	return func() tea.Msg {
		return shared.NodeActionRequestMsg{
			Action:         action,
			NodeHostnames:  hostnames,
			NodeNames:      names,
			IsControlPlane: isCP,
		}
	}
}

func (m Model) fetchMembers() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		nodes, err := cluster.GetMembers(ctx, client)
		return membersLoadedMsg{nodes: nodes, err: err}
	}
}

func (m Model) fetchServicesForDetail() tea.Cmd {
	client := m.client
	if client == nil || client.C == nil || m.cursor >= len(m.filtered) {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.C.ServiceList(ctx)
		if err != nil {
			return servicesLoadedMsg{err: err}
		}

		byNode := make(map[string][]serviceInfo)
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			hostname := nodeMsg.GetMetadata().GetHostname()
			for _, svc := range nodeMsg.GetServices() {
				health := "?"
				if svc.GetHealth() != nil {
					if svc.GetHealth().GetUnknown() {
						health = "?"
					} else if svc.GetHealth().GetHealthy() {
						health = "OK"
					} else {
						health = "Failed"
					}
				}
				byNode[hostname] = append(byNode[hostname], serviceInfo{
					ID:     svc.GetId(),
					State:  svc.GetState(),
					Health: health,
				})
			}
		}
		return servicesLoadedMsg{servicesByNode: byNode}
	}
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
