// Package dashboard provides the cluster dashboard tab view.
package dashboard

import (
	"context"
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
)

// Expected service sets by node type.
var (
	cpServices     = []string{"apid", "auditd", "containerd", "cri", "dashboard", "etcd", "kubelet", "machined", "syslogd", "trustd", "udevd"}
	workerServices = []string{"apid", "auditd", "containerd", "cri", "dashboard", "kubelet", "machined", "syslogd", "udevd"}
	allServices    = []string{"apid", "auditd", "containerd", "cri", "dashboard", "etcd", "kubelet", "machined", "syslogd", "trustd", "udevd"}
)

type serviceRow struct {
	ServiceID string
	State     string
	Health    string
}

type memStats struct {
	TotalKB     uint64
	AvailableKB uint64
}

type eventRow struct {
	Node    string
	Actor   string
	Message string
}

// Internal messages for data loading.
type membersLoadedMsg struct {
	nodes []cluster.NodeInfo
	err   error
}

type servicesLoadedMsg struct {
	servicesByNode map[string][]serviceRow
	err            error
}

type memoryLoadedMsg struct {
	memoryByNode map[string]memStats
	err          error
}

type eventsLoadedMsg struct {
	events []eventRow
	err    error
}

// Model is the dashboard view model.
type Model struct {
	client          *talos.Client
	nodes           []cluster.NodeInfo
	servicesByNode  map[string][]serviceRow
	memoryByNode    map[string]memStats
	events          []eventRow
	loading         bool
	err             error
	width           int
	height          int
	refreshInterval time.Duration
	lastRefresh     time.Time
	eventScroll     int
	followEvents    bool
}

// New creates a new dashboard model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		refreshInterval: refreshInterval,
		servicesByNode:  make(map[string][]serviceRow),
		memoryByNode:    make(map[string]memStats),
		loading:         true,
		followEvents:    true,
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
		switch {
		case key.Matches(msg, shared.Keys.LogFollow):
			m.followEvents = !m.followEvents
		case key.Matches(msg, shared.Keys.Down):
			m.eventScroll++
			m.followEvents = false
		case key.Matches(msg, shared.Keys.Up):
			if m.eventScroll > 0 {
				m.eventScroll--
			}
			m.followEvents = false
		}

	case shared.TickMsg:
		return m, m.ForceRefresh()

	case membersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.nodes = msg.nodes
		}
		m.loading = false
		m.lastRefresh = time.Now()

	case servicesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.servicesByNode = msg.servicesByNode
		}

	case memoryLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.memoryByNode = msg.memoryByNode
		}

	case eventsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.events = msg.events
		}
	}

	return m, nil
}

// View renders the dashboard.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.loading && len(m.nodes) == 0 {
		return shared.StyleMuted.Render("  Loading cluster data...")
	}

	var sections []string

	// Header
	header := m.renderHeader()
	sections = append(sections, header)

	if m.err != nil {
		sections = append(sections, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	// Height allocation
	headerHeight := strings.Count(header, "\n") + 1
	remaining := m.height - headerHeight
	if m.err != nil {
		remaining -= 2
	}

	nodeHeight := len(m.nodes) + 2
	if nodeHeight > remaining*30/100 {
		nodeHeight = remaining * 30 / 100
	}
	if nodeHeight < 3 {
		nodeHeight = 3
	}

	serviceHeight := len(allServices) + 2
	if serviceHeight > remaining*30/100 {
		serviceHeight = remaining * 30 / 100
	}
	if serviceHeight < 3 {
		serviceHeight = 3
	}

	eventHeight := remaining - nodeHeight - serviceHeight
	if eventHeight < 3 {
		eventHeight = 3
	}

	// Node table
	sections = append(sections, m.renderNodeTable(nodeHeight))

	// Service matrix
	sections = append(sections, m.renderServiceMatrix(serviceHeight))

	// Events
	sections = append(sections, m.renderEvents(eventHeight))

	content := strings.Join(sections, "\n")

	// Truncate to height
	lines := strings.Split(content, "\n")
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
	return "F:follow events  ↑↓:scroll events  ctrl+r:refresh"
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return tea.Batch(
		m.fetchMembers(),
		m.fetchServices(),
		m.fetchMemory(),
		m.fetchEvents(),
	)
}

// --- Data fetching ---

func (m Model) fetchMembers() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		nodes, err := cluster.GetMembers(ctx, client)
		return membersLoadedMsg{nodes: nodes, err: err}
	}
}

func (m Model) fetchServices() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return servicesLoadedMsg{servicesByNode: make(map[string][]serviceRow)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.C.ServiceList(ctx)
		if err != nil {
			return servicesLoadedMsg{err: err}
		}

		byNode := make(map[string][]serviceRow)
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			hostname := nodeMsg.GetMetadata().GetHostname()
			if hostname == "" {
				continue
			}
			var svcs []serviceRow
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
				svcs = append(svcs, serviceRow{
					ServiceID: svc.GetId(),
					State:     svc.GetState(),
					Health:    health,
				})
			}
			byNode[hostname] = svcs
		}
		return servicesLoadedMsg{servicesByNode: byNode}
	}
}

func (m Model) fetchMemory() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return memoryLoadedMsg{memoryByNode: make(map[string]memStats)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.C.Memory(ctx)
		if err != nil {
			return memoryLoadedMsg{err: err}
		}

		byNode := make(map[string]memStats)
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil || nodeMsg.GetMeminfo() == nil {
				continue
			}
			hostname := nodeMsg.GetMetadata().GetHostname()
			if hostname == "" {
				continue
			}
			byNode[hostname] = memStats{
				TotalKB:     nodeMsg.GetMeminfo().GetMemtotal(),
				AvailableKB: nodeMsg.GetMeminfo().GetMemavailable(),
			}
		}
		return memoryLoadedMsg{memoryByNode: byNode}
	}
}

func (m Model) fetchEvents() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return eventsLoadedMsg{events: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		stream, err := client.C.Events(ctx, talosclient.WithTailEvents(20))
		if err != nil {
			return eventsLoadedMsg{err: err}
		}

		var events []eventRow
		for i := 0; i < 40; i++ { // read up to 40 to handle multi-node responses, deduplicate later
			ev, err := stream.Recv()
			if err != nil {
				break
			}
			hostname := ""
			if ev.GetMetadata() != nil {
				hostname = ev.GetMetadata().GetHostname()
			}
			actor := ev.GetActorId()
			message := ""
			if ev.GetData() != nil {
				message = ev.GetData().GetTypeUrl()
			}
			// Extract type name from URL
			if idx := strings.LastIndex(message, "."); idx >= 0 {
				message = message[idx+1:]
			}

			events = append(events, eventRow{
				Node:    shortenHostname(hostname),
				Actor:   actor,
				Message: message,
			})
		}

		// Keep last 20
		if len(events) > 20 {
			events = events[len(events)-20:]
		}

		return eventsLoadedMsg{events: events}
	}
}

// --- Rendering helpers ---

func (m Model) renderHeader() string {
	cpCount := 0
	workerCount := 0
	version := ""
	for _, n := range m.nodes {
		if n.IsControlPlane() {
			cpCount++
		} else {
			workerCount++
		}
		if version == "" && n.TalosVersion != "" {
			version = n.TalosVersion
		}
	}

	contextName := ""
	if m.client != nil {
		contextName = m.client.ContextName
	}

	header := shared.StyleHeader.Render(fmt.Sprintf(
		"  CLUSTER: %s  %d nodes  %d CP / %d Worker  %s",
		contextName, len(m.nodes), cpCount, workerCount, version,
	))

	sep := shared.StyleMuted.Render("  " + strings.Repeat("\u2550", min(m.width-4, 80)))

	return header + "\n" + sep
}

// RenderNodeTable renders the node summary section. Exported for testing.
func RenderNodeTable(nodes []cluster.NodeInfo, servicesByNode map[string][]serviceRow, memoryByNode map[string]memStats, width, maxLines int) string {
	if len(nodes) == 0 {
		return shared.StyleMuted.Render("  No nodes found")
	}

	// Column headers
	header := fmt.Sprintf("  %-24s %-6s %-8s %-6s",
		"NODE", "TYPE", "HEALTH", "MEM%")
	lines := []string{shared.StyleHeader.Render(header)}

	for i, n := range nodes {
		if i+2 >= maxLines {
			break
		}
		typeStr := "Worker"
		if n.IsControlPlane() {
			typeStr = "CP"
		}

		// Determine health from services
		healthIcon := shared.StatusIcon("Running")
		healthStyle := shared.StyleSuccess
		if svcs, ok := servicesByNode[n.Hostname]; ok {
			for _, s := range svcs {
				if s.Health == "Failed" {
					healthIcon = shared.StatusIcon("Failed")
					healthStyle = shared.StyleError
					break
				}
			}
		}

		// Memory percentage
		memStr := "N/A"
		if mem, ok := memoryByNode[n.Hostname]; ok && mem.TotalKB > 0 {
			used := mem.TotalKB - mem.AvailableKB
			pct := float64(used) / float64(mem.TotalKB) * 100
			memStr = fmt.Sprintf("%.0f%%", pct)
		}

		row := fmt.Sprintf("  %-24s %-6s %s       %-6s",
			truncate(n.Hostname, 24),
			typeStr,
			healthStyle.Render(healthIcon),
			memStr,
		)
		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderNodeTable(maxLines int) string {
	return RenderNodeTable(m.nodes, m.servicesByNode, m.memoryByNode, m.width, maxLines)
}

// RenderServiceMatrix renders the service status matrix. Exported for testing.
func RenderServiceMatrix(nodes []cluster.NodeInfo, servicesByNode map[string][]serviceRow, maxLines int) string {
	if len(nodes) == 0 {
		return ""
	}

	// Build service lookup per node
	svcByNodeAndID := make(map[string]map[string]serviceRow)
	for hostname, svcs := range servicesByNode {
		svcMap := make(map[string]serviceRow)
		for _, s := range svcs {
			svcMap[s.ServiceID] = s
		}
		svcByNodeAndID[hostname] = svcMap
	}

	// Build a set of expected services per node type
	cpSet := make(map[string]bool)
	for _, s := range cpServices {
		cpSet[s] = true
	}
	workerSet := make(map[string]bool)
	for _, s := range workerServices {
		workerSet[s] = true
	}

	// Header: short node names
	shortNames := make([]string, len(nodes))
	for i, n := range nodes {
		shortNames[i] = shortenHostname(n.Hostname)
	}

	nodeColWidth := 6
	header := fmt.Sprintf("  %-14s", "SERVICE")
	for _, sn := range shortNames {
		header += fmt.Sprintf("%-*s", nodeColWidth, truncate(sn, nodeColWidth-1))
	}
	lines := []string{shared.StyleHeader.Render(header)}

	for idx, svcName := range allServices {
		if idx+2 >= maxLines {
			break
		}
		row := fmt.Sprintf("  %-14s", svcName)
		for _, n := range nodes {
			expectedSet := workerSet
			if n.IsControlPlane() {
				expectedSet = cpSet
			}

			if !expectedSet[svcName] {
				row += shared.StyleMuted.Render(fmt.Sprintf("%-*s", nodeColWidth, "-"))
				continue
			}

			svcMap := svcByNodeAndID[n.Hostname]
			if svc, ok := svcMap[svcName]; ok {
				icon := shared.StatusIcon("Running")
				style := shared.StyleSuccess
				if svc.State != "Running" || svc.Health == "Failed" {
					icon = shared.StatusIcon("Failed")
					style = shared.StyleError
				}
				row += style.Render(fmt.Sprintf("%-*s", nodeColWidth, icon))
			} else {
				row += shared.StyleMuted.Render(fmt.Sprintf("%-*s", nodeColWidth, "?"))
			}
		}
		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderServiceMatrix(maxLines int) string {
	return RenderServiceMatrix(m.nodes, m.servicesByNode, maxLines)
}

func (m Model) renderEvents(maxLines int) string {
	title := shared.StyleHeader.Render("  RECENT EVENTS")
	if m.followEvents {
		title += shared.StyleMuted.Render(" (following)")
	}
	lines := []string{title}

	if len(m.events) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No events"))
		return strings.Join(lines, "\n")
	}

	startIdx := 0
	visible := maxLines - 1
	if visible < 1 {
		visible = 1
	}

	if m.followEvents {
		startIdx = len(m.events) - visible
	} else {
		startIdx = m.eventScroll
	}
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(m.events)-1 {
		startIdx = len(m.events) - 1
	}

	for i := startIdx; i < len(m.events) && i < startIdx+visible; i++ {
		ev := m.events[i]
		nodeColor := nodeColorFor(ev.Node)
		nodeTag := lipgloss.NewStyle().Foreground(nodeColor).Render(fmt.Sprintf("[%-8s]", ev.Node))
		actorTag := shared.StyleMuted.Render(fmt.Sprintf("[%-10s]", truncate(ev.Actor, 10)))
		line := fmt.Sprintf("  %s%s %s", nodeTag, actorTag, ev.Message)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// --- Utility functions ---

func shortenHostname(hostname string) string {
	// Try to shorten: "my-cluster-cp-1" -> "cp-1"
	parts := strings.Split(hostname, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "-")
	}
	return hostname
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "\u2026"
}

func nodeColorFor(hostname string) color.Color {
	if len(shared.NodeColors) == 0 {
		return lipgloss.Color("#839496")
	}
	h := 0
	for _, c := range hostname {
		h += int(c)
	}
	idx := h % len(shared.NodeColors)
	return shared.NodeColors[idx]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SortNodesByHostname sorts nodes alphabetically. Exported for testing.
func SortNodesByHostname(nodes []cluster.NodeInfo) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Hostname < nodes[j].Hostname
	})
}
