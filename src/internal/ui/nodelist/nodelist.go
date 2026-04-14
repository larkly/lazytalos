// Package nodelist provides the cluster nodes tab view with space-select and detail.
package nodelist

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"google.golang.org/grpc"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/resources"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

type sortField int

const (
	sortByHostname sortField = iota
	sortByType
	sortByHealth
	sortByCPU
	sortByMemory
	sortFieldMax
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

type memoryLoadedMsg struct {
	memoryByNode map[string]shared.MemStats
	err          error
}

type cpuLoadedMsg struct {
	cpuByNode map[string]resources.CPUStats
	err       error
}

// Detail log streaming types.
type detailLogLineMsg struct {
	service string
	text    string
	isErr   bool
}

type detailLogEndedMsg struct {
	service string
	err     error // nil for clean close (EOF / context cancel)
}

type detailLogLine struct {
	service string
	text    string
	isErr   bool
	t       time.Time
}

type detailStream struct {
	cancel context.CancelFunc
	stream grpc.ServerStreamingClient[common.Data]
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
	sortBy          sortField
	refreshInterval time.Duration
	memoryByNode    map[string]shared.MemStats
	cpuByNode       map[string]resources.CPUStats

	// Detail log streaming
	detailLogs    []detailLogLine
	detailStreams map[string]detailStream // service -> stream
	detailScroll  int
	detailFollow  bool
}

// New creates a new node list model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		selected:        make(map[string]bool),
		loading:         true,
		refreshInterval: refreshInterval,
		memoryByNode:    make(map[string]shared.MemStats),
		cpuByNode:       make(map[string]resources.CPUStats),
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
		return m, m.ForceRefresh()

	case membersLoadedMsg:
		if msg.err != nil && len(msg.nodes) == 0 {
			m.err = msg.err
		} else {
			m.err = nil
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

	case memoryLoadedMsg:
		if msg.err == nil {
			m.memoryByNode = msg.memoryByNode
		}

	case cpuLoadedMsg:
		if msg.err == nil {
			m.cpuByNode = msg.cpuByNode
		}

	case detailLogLineMsg:
		if m.detailView {
			m.detailLogs = append(m.detailLogs, detailLogLine{
				service: msg.service,
				text:    msg.text,
				isErr:   msg.isErr,
				t:       time.Now(),
			})
			const maxDetailLines = 1000
			if len(m.detailLogs) > maxDetailLines {
				m.detailLogs = m.detailLogs[len(m.detailLogs)-maxDetailLines:]
			}
			// Chain next read from the same stream
			if s, ok := m.detailStreams[msg.service]; ok {
				return m, awaitDetailLogLine(s.stream, msg.service)
			}
		}

	case detailLogEndedMsg:
		delete(m.detailStreams, msg.service)
		text, isErr := formatDetailStreamEnd(msg.err)
		m.detailLogs = append(m.detailLogs, detailLogLine{
			service: msg.service,
			text:    text,
			isErr:   isErr,
			t:       time.Now(),
		})
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
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.adjustScroll()
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
			m.detailLogs = nil
			m.detailStreams = make(map[string]detailStream)
			m.detailScroll = 0
			m.detailFollow = true
			return m, tea.Batch(m.fetchServicesForDetail(), m.startDetailLogStreams())
		}
	case key.Matches(msg, shared.Keys.Filter):
		m.filterActive = true
	case key.Matches(msg, shared.Keys.Reboot):
		return m, m.emitNodeAction("reboot")
	case key.Matches(msg, shared.Keys.Shutdown):
		return m, m.emitNodeAction("shutdown")
	case key.Matches(msg, shared.Keys.UpgradeCluster):
		var hostnames []string
		selected := m.SelectedNodes()
		if len(m.selected) == 0 && len(m.nodes) > 0 {
			// no selection → use all nodes
			for _, n := range m.nodes {
				hostnames = append(hostnames, n.Hostname)
			}
		} else {
			for _, n := range selected {
				hostnames = append(hostnames, n.Hostname)
			}
		}
		if len(hostnames) > 0 {
			return m, func() tea.Msg {
				return shared.UpgradeRequestMsg{Nodes: hostnames}
			}
		}
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
	case key.Matches(msg, shared.Keys.Sort):
		m.sortBy = (m.sortBy + 1) % sortFieldMax
		m.sortData()
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
		m.CancelDetailStreams()
		m.detailView = false
	case key.Matches(msg, shared.Keys.LogFollow):
		m.detailFollow = !m.detailFollow
	case key.Matches(msg, shared.Keys.Down):
		m.detailScroll++
		m.detailFollow = false
	case key.Matches(msg, shared.Keys.Up):
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		m.detailFollow = false
	case key.Matches(msg, shared.Keys.PageDown):
		m.detailScroll += 10
		m.detailFollow = false
	case key.Matches(msg, shared.Keys.PageUp):
		m.detailScroll -= 10
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
		m.detailFollow = false
	case key.Matches(msg, shared.Keys.ResetNode):
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			n := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return shared.NodeResetRequestMsg{Node: n.Hostname, IsControlPlane: n.IsControlPlane()}
			}
		}
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = make([]cluster.NodeInfo, len(m.nodes))
		copy(m.filtered, m.nodes)
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
	m.sortData()
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) sortData() {
	switch m.sortBy {
	case sortByHostname:
		slices.SortFunc(m.filtered, func(a, b cluster.NodeInfo) int {
			return cmp.Compare(a.Hostname, b.Hostname)
		})
	case sortByType:
		slices.SortFunc(m.filtered, func(a, b cluster.NodeInfo) int {
			return cmp.Compare(a.MachineType, b.MachineType)
		})
	case sortByHealth:
		slices.SortFunc(m.filtered, func(a, b cluster.NodeInfo) int {
			aH := 0
			if a.Healthy {
				aH = 1
			}
			bH := 0
			if b.Healthy {
				bH = 1
			}
			return cmp.Compare(aH, bH)
		})
	case sortByCPU:
		slices.SortFunc(m.filtered, func(a, b cluster.NodeInfo) int {
			aV := 0.0
			if cs, ok := m.cpuByNode[a.Hostname]; ok {
				aV = cs.UsagePercent
			}
			bV := 0.0
			if cs, ok := m.cpuByNode[b.Hostname]; ok {
				bV = cs.UsagePercent
			}
			return cmp.Compare(bV, aV) // descending — highest CPU first
		})
	case sortByMemory:
		slices.SortFunc(m.filtered, func(a, b cluster.NodeInfo) int {
			aV := 0.0
			if mem, ok := m.memoryByNode[a.Hostname]; ok && mem.TotalKB > 0 {
				aV = float64(mem.TotalKB-mem.AvailableKB) / float64(mem.TotalKB)
			}
			bV := 0.0
			if mem, ok := m.memoryByNode[b.Hostname]; ok && mem.TotalKB > 0 {
				bV = float64(mem.TotalKB-mem.AvailableKB) / float64(mem.TotalKB)
			}
			return cmp.Compare(bV, aV) // descending — highest memory first
		})
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
	v := m.height - 4 // title + filter + column header + separator
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

	var b strings.Builder

	// Title line with count
	title := shared.StyleTitle.Render("Nodes")
	count := shared.StyleMuted.Render(fmt.Sprintf(" (%d)", len(m.filtered)))
	if len(m.selected) > 0 {
		count += shared.StyleMuted.Render(fmt.Sprintf("  %d selected", len(m.selected)))
	}
	b.WriteString(title + count + "\n")

	// Filter bar
	if m.filterActive {
		b.WriteString("  / " + shared.StyleWarning.Render(m.filter+"_") + "\n")
	} else if m.filter != "" {
		b.WriteString(shared.StyleMuted.Render(fmt.Sprintf("  filter: %s", m.filter)) + "\n")
	} else {
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString(shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)) + "\n")
	}

	// Column widths — adapt to terminal width.
	// Fixed columns: prefix(2) + type(14) + version(10) + health(3) + uptime(7) + gaps(6)
	const fixedW = 2 + 14 + 10 + 3 + 7 + 6
	flexible := m.width - fixedW
	// Distribute: name(20) + cpubar + membar, then IP/IPv6 if room
	nameW := 20
	ipW := 0
	ip6W := 0
	barSpace := flexible - nameW
	if barSpace > 60 {
		excess := barSpace - 60
		barSpace = 60
		// First priority: IPv4 column (16 chars)
		if excess >= 16 {
			ipW = 16
			excess -= 16
		}
		// Second priority: give leftover to hostname
		if excess > 0 && nameW < 30 {
			add := excess
			if nameW+add > 30 {
				add = 30 - nameW
			}
			nameW += add
			excess -= add
		}
		// Third priority: IPv6 column if there's still room (needs ~40 chars)
		if excess >= 25 {
			ip6W = excess
			if ip6W > 42 {
				ip6W = 42
			}
		}
	}
	if barSpace < 16 {
		barSpace = 16
		nameW = flexible - barSpace
		if nameW < 10 {
			nameW = 10
		}
	}
	// Split bar space evenly between CPU and memory
	cpuBarW := barSpace / 2
	memBarW := barSpace - cpuBarW
	if cpuBarW < 8 {
		cpuBarW = 8
	}
	if memBarW < 8 {
		memBarW = 8
	}

	// Sort indicator
	sortIndicator := func(col sortField) string {
		if m.sortBy == col {
			return " ▲"
		}
		return ""
	}

	// Header
	header := fmt.Sprintf("  %-*s %-14s %-10s %-3s %-*s %-*s %-7s",
		nameW, "HOSTNAME"+sortIndicator(sortByHostname),
		"TYPE"+sortIndicator(sortByType),
		"VERSION",
		"●",
		cpuBarW, "CPU"+sortIndicator(sortByCPU),
		memBarW, "MEMORY"+sortIndicator(sortByMemory),
		"UPTIME")
	if ipW > 0 {
		header += fmt.Sprintf(" %-*s", ipW, "IPv4")
	}
	if ip6W > 0 {
		header += fmt.Sprintf(" %-*s", ip6W, "IPv6")
	}
	b.WriteString(shared.StyleHeader.Render(header) + "\n")

	// Separator
	b.WriteString(shared.StyleMuted.Render(strings.Repeat("─", m.width)) + "\n")

	// Rows
	visible := m.visibleRows()
	endIdx := m.scrollOff + visible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	for i := m.scrollOff; i < endIdx; i++ {
		n := m.filtered[i]
		isCursor := i == m.cursor
		isSelected := m.selected[n.Hostname]

		// Selection marker
		prefix := "  "
		if isSelected {
			prefix = "● "
		}

		typeStr := "worker"
		if n.IsControlPlane() {
			typeStr = "controlplane"
		}

		healthIcon := shared.StatusIcon("Running")
		healthStyle := shared.StyleSuccess
		if !n.Healthy {
			healthIcon = shared.StatusIcon("Failed")
			healthStyle = shared.StyleError
		}

		// CPU bar — pad to cpuBarW visual width
		cpuBar := shared.StyleMuted.Render(fmt.Sprintf("%-*s", cpuBarW, " N/A"))
		if cs, ok := m.cpuByNode[n.Hostname]; ok {
			rendered := shared.RenderCPUBar(cs.UsagePercent, cpuBarW)
			visW := lipgloss.Width(rendered)
			if visW < cpuBarW {
				rendered += strings.Repeat(" ", cpuBarW-visW)
			}
			cpuBar = rendered
		}

		// Memory bar — pad to memBarW visual width
		memBar := shared.StyleMuted.Render(fmt.Sprintf("%-*s", memBarW, " N/A"))
		if mem, ok := m.memoryByNode[n.Hostname]; ok && mem.TotalKB > 0 {
			used := mem.TotalKB - mem.AvailableKB
			pct := float64(used) / float64(mem.TotalKB)
			rendered := shared.RenderMemBar(pct, memBarW)
			visW := lipgloss.Width(rendered)
			if visW < memBarW {
				rendered += strings.Repeat(" ", memBarW-visW)
			}
			memBar = rendered
		}

		// Uptime
		uptimeStr := shared.StyleMuted.Render("   --  ")
		if cs, ok := m.cpuByNode[n.Hostname]; ok && !cs.BootTime.IsZero() {
			uptimeStr = fmt.Sprintf("%-7s", shared.FormatUptime(time.Since(cs.BootTime)))
		}

		// Split addresses into IPv4 and IPv6
		var ip4, ip6 string
		for _, a := range n.Addresses {
			if strings.Contains(a, ":") {
				if ip6 == "" {
					ip6 = a
				}
			} else {
				if ip4 == "" {
					ip4 = a
				}
			}
		}

		// Build plain row for alignment, then apply styling
		nameStr := shared.Truncate(shared.ShortenHostname(n.Hostname), nameW)
		row := fmt.Sprintf("%-*s %-14s %-10s %s %s %s %s",
			nameW, nameStr,
			typeStr,
			n.TalosVersion,
			healthStyle.Render(healthIcon),
			cpuBar,
			memBar,
			uptimeStr)
		if ipW > 0 {
			row += fmt.Sprintf(" %-*s", ipW, shared.Truncate(ip4, ipW))
		}
		if ip6W > 0 {
			row += fmt.Sprintf(" %-*s", ip6W, shared.Truncate(ip6, ip6W))
		}

		// Apply row background and styling
		hasBg := isCursor || isSelected
		prefixStyle := lipgloss.NewStyle()
		rowStyle := lipgloss.NewStyle()

		if isCursor {
			bg := lipgloss.Color("#073642")
			prefixStyle = prefixStyle.Background(bg)
			rowStyle = rowStyle.Background(bg).Bold(true)
		} else if isSelected {
			bg := lipgloss.Color("#1a1a2e")
			prefixStyle = prefixStyle.Background(bg).Foreground(shared.ColorPrimary)
			rowStyle = rowStyle.Background(bg)
		}

		fullRow := prefixStyle.Render(prefix) + rowStyle.Render(row)

		// Pad to full width for consistent background
		if hasBg {
			rowW := lipgloss.Width(fullRow)
			if rowW < m.width {
				fullRow += rowStyle.Render(strings.Repeat(" ", m.width-rowW))
			}
		}

		b.WriteString(fullRow + "\n")
	}

	return b.String()
}

func (m Model) viewDetail() string {
	n := m.filtered[m.cursor]
	var lines []string

	lines = append(lines, shared.StyleHeader.Render(fmt.Sprintf("  Node: %s", shared.ShortenHostname(n.Hostname))))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Type:"), n.MachineType))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Talos Version:"), n.TalosVersion))
	lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Healthy:"), fmt.Sprintf("%v", n.Healthy)))

	// CPU
	if cs, ok := m.cpuByNode[n.Hostname]; ok {
		lines = append(lines, fmt.Sprintf("  %-20s %.0f%%", shared.StyleLabel.Render("CPU:"), cs.UsagePercent*100))
	}

	// Memory
	if mem, ok := m.memoryByNode[n.Hostname]; ok && mem.TotalKB > 0 {
		used := mem.TotalKB - mem.AvailableKB
		pct := float64(used) / float64(mem.TotalKB)
		lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Memory:"), shared.RenderMemBar(pct, 20)))
	}

	// Uptime
	if cs, ok := m.cpuByNode[n.Hostname]; ok && !cs.BootTime.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-20s %s", shared.StyleLabel.Render("Uptime:"), shared.FormatUptime(time.Since(cs.BootTime))))
	}

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

	// Log pane separator
	lines = append(lines, "")
	followTag := ""
	if m.detailFollow {
		followTag = shared.StyleMuted.Render(" (following)")
	}
	lines = append(lines, shared.StyleLabel.Render("  Logs:")+followTag)
	lines = append(lines, shared.StyleMuted.Render("  "+strings.Repeat("─", m.width-4)))

	// Calculate available log lines
	infoLines := len(lines)
	logViewH := m.height - infoLines - 1 // -1 for hint line
	if logViewH < 3 {
		logViewH = 3
	}

	if len(m.detailLogs) == 0 {
		lines = append(lines, shared.StyleMuted.Render("    Waiting for logs..."))
		for len(lines) < m.height-1 {
			lines = append(lines, "")
		}
	} else {
		// Determine scroll window
		startIdx := 0
		if m.detailFollow {
			startIdx = len(m.detailLogs) - logViewH
		} else {
			startIdx = m.detailScroll
		}
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(m.detailLogs)-1 {
			startIdx = len(m.detailLogs) - 1
		}

		for i := startIdx; i < len(m.detailLogs) && i < startIdx+logViewH; i++ {
			l := m.detailLogs[i]
			svcColor := detailLogColorFor(l.service)
			ts := shared.StyleMuted.Render(l.t.Format("15:04:05"))
			svcTag := lipgloss.NewStyle().Foreground(svcColor).Render(fmt.Sprintf("[%-10s]", shared.Truncate(l.service, 10)))
			text := l.text
			maxTextW := m.width - 24
			if maxTextW > 0 && len(text) > maxTextW {
				text = text[:maxTextW]
			}
			line := fmt.Sprintf("  %s %s %s", ts, svcTag, text)
			if l.isErr {
				line = shared.StyleError.Render(line)
			}
			lines = append(lines, line)
		}
		for len(lines) < m.height-1 {
			lines = append(lines, "")
		}
	}

	lines = append(lines, shared.StyleMuted.Render("  esc:back  ↑↓:scroll logs  F:follow  ctrl+x:reset"))

	if len(lines) > m.height {
		lines = lines[:m.height]
	}
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
		return "esc:back  ↑↓:scroll logs  F:follow  ctrl+x:reset"
	}
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	sortLabel := [sortFieldMax]string{"hostname", "type", "health", "cpu", "memory"}
	return fmt.Sprintf("space:select  A:all  enter:detail  /:filter  s:sort(%s)  y:copy IP  Y:copy endpoint  ctrl+o:reboot  ctrl+d:shutdown  ctrl+u:upgrade", sortLabel[m.sortBy])
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return tea.Batch(
		m.fetchMembers(),
		m.fetchMemory(),
		m.fetchCPU(),
	)
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

// AllNodes returns the full list of loaded nodes.
func (m Model) AllNodes() []cluster.NodeInfo {
	return m.nodes
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
		if err != nil || len(nodes) == 0 || client == nil || client.C == nil {
			return membersLoadedMsg{nodes: nodes, err: err}
		}

		// Use NodeTargets to determine which nodes are reachable
		reachableAddrs, _ := cluster.NodeTargets(ctx, client)
		reachable := make(map[string]bool)
		for _, a := range reachableAddrs {
			reachable[a] = true
		}
		for i, n := range nodes {
			nodes[i].Healthy = false
			for _, a := range n.Addresses {
				if reachable[a] {
					nodes[i].Healthy = true
					break
				}
			}
		}

		return membersLoadedMsg{nodes: nodes, err: err}
	}
}

func (m Model) fetchServicesForDetail() tea.Cmd {
	client := m.client
	if client == nil || client.C == nil || m.cursor >= len(m.filtered) {
		return nil
	}
	// Target the specific node for this detail view
	node := m.filtered[m.cursor]
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Use the node's first address for WithNodes targeting
		target := node.Hostname
		if len(node.Addresses) > 0 {
			target = node.Addresses[0]
		}
		nodeCtx := talosclient.WithNodes(ctx, target)
		resp, err := client.C.ServiceList(nodeCtx)
		if err != nil {
			return servicesLoadedMsg{err: err}
		}

		byNode := make(map[string][]serviceInfo)
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			// Key by the node's hostname so the lookup matches
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
				byNode[node.Hostname] = append(byNode[node.Hostname], serviceInfo{
					ID:     svc.GetId(),
					State:  svc.GetState(),
					Health: health,
				})
			}
		}
		return servicesLoadedMsg{servicesByNode: byNode}
	}
}

func (m Model) fetchMemory() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return memoryLoadedMsg{memoryByNode: make(map[string]shared.MemStats)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		stats, err := resources.ListMemStats(ctx, client)
		byNode := make(map[string]shared.MemStats, len(stats))
		for _, s := range stats {
			byNode[s.NodeHostname] = s
		}
		return memoryLoadedMsg{memoryByNode: byNode, err: err}
	}
}

func (m Model) fetchCPU() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return cpuLoadedMsg{cpuByNode: make(map[string]resources.CPUStats)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stats, err := resources.ListCPUStats(ctx, client)
		byNode := make(map[string]resources.CPUStats, len(stats))
		for _, s := range stats {
			byNode[s.NodeHostname] = s
		}
		return cpuLoadedMsg{cpuByNode: byNode, err: err}
	}
}

// defaultDetailServices are the services to stream logs for in node detail view.
var defaultDetailServices = []string{
	"kubelet", "etcd", "apid", "machined", "containerd",
}

func (m *Model) startDetailLogStreams() tea.Cmd {
	if m.client == nil || m.client.C == nil || m.cursor >= len(m.filtered) {
		return nil
	}
	node := m.filtered[m.cursor]
	target := node.Hostname
	if len(node.Addresses) > 0 {
		target = node.Addresses[0]
	}

	var cmds []tea.Cmd
	for _, svc := range defaultDetailServices {
		if _, exists := m.detailStreams[svc]; exists {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		nodeCtx := talosclient.WithNodes(ctx, target)

		var tailLines int32 = 20
		stream, err := m.client.C.Logs(nodeCtx, "system", common.ContainerDriver_CONTAINERD, svc, true, tailLines)
		if err != nil {
			cancel()
			continue
		}
		m.detailStreams[svc] = detailStream{cancel: cancel, stream: stream}
		cmds = append(cmds, awaitDetailLogLine(stream, svc))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// CancelDetailStreams cancels any live per-service log tails held by the
// detail view. Safe to call from app-level teardown (e.g. context switch)
// to prevent goroutine leaks after the underlying client is closed.
func (m *Model) CancelDetailStreams() {
	for svc, s := range m.detailStreams {
		s.cancel()
		delete(m.detailStreams, svc)
	}
	m.detailLogs = nil
}

// formatDetailStreamEnd renders the synthetic "stream ended" line shown
// in the node-detail log pane. Clean closes stay quiet; real errors are
// surfaced so the user can tell a drained tail apart from a broken one.
func formatDetailStreamEnd(err error) (text string, isErr bool) {
	switch {
	case err == nil, errors.Is(err, io.EOF), errors.Is(err, context.Canceled):
		return "--- stream ended ---", false
	default:
		return "--- stream ended: " + err.Error() + " ---", true
	}
}

func awaitDetailLogLine(stream grpc.ServerStreamingClient[common.Data], svc string) tea.Cmd {
	return func() tea.Msg {
		data, err := stream.Recv()
		if err != nil {
			return detailLogEndedMsg{service: svc, err: err}
		}
		text := string(data.GetBytes())
		text = strings.TrimRight(text, "\n")
		return detailLogLineMsg{service: svc, text: text}
	}
}

func detailLogColorFor(svc string) color.Color {
	if len(shared.NodeColors) == 0 {
		return lipgloss.Color("#839496")
	}
	h := 0
	for _, c := range svc {
		h += int(c)
	}
	return shared.NodeColors[h%len(shared.NodeColors)]
}
