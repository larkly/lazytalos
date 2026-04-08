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
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/resources"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
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
	Time    time.Time
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

type diagnosticsLoadedMsg struct {
	diagnostics []resources.DiagnosticEntry
	err         error
}

type cpuLoadedMsg struct {
	cpuByNode map[string]resources.CPUStats
	err       error
}

// Model is the dashboard view model.
type Model struct {
	client          *talos.Client
	nodes           []cluster.NodeInfo
	servicesByNode  map[string][]serviceRow
	memoryByNode    map[string]memStats
	cpuByNode       map[string]resources.CPUStats
	events          []eventRow
	diagnostics     []resources.DiagnosticEntry
	loading         bool
	errs            []error
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
		cpuByNode:       make(map[string]resources.CPUStats),
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
			m.appendErr(msg.err)
		} else {
			m.nodes = msg.nodes
			m.clearErr()
		}
		m.loading = false
		m.lastRefresh = time.Now()

	case servicesLoadedMsg:
		if msg.err != nil {
			m.appendErr(msg.err)
		} else {
			m.servicesByNode = msg.servicesByNode
		}

	case memoryLoadedMsg:
		if msg.err != nil {
			m.appendErr(msg.err)
		} else {
			m.memoryByNode = msg.memoryByNode
		}

	case eventsLoadedMsg:
		if msg.err != nil {
			m.appendErr(msg.err)
		} else {
			m.events = msg.events
		}

	case diagnosticsLoadedMsg:
		if msg.err != nil {
			m.appendErr(msg.err)
		} else {
			m.diagnostics = msg.diagnostics
		}

	case cpuLoadedMsg:
		if msg.err != nil {
			m.appendErr(msg.err)
		} else {
			m.cpuByNode = msg.cpuByNode
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

	// Panel border style.
	// lipgloss v2: Width() is the TOTAL width including borders.
	// A rounded border adds 1 col each side = 2 total.
	panelBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(shared.ColorSecondary)

	// Two side-by-side panels: left gets half, right gets remainder (handles odd widths).
	leftW := m.width / 2
	rightW := m.width - leftW
	fullW := m.width

	// --- Top row: Cluster Status (left) + Node Health (right) ---
	topInnerH := max(len(m.nodes)+2, 8)
	if topInnerH > m.height*40/100 {
		topInnerH = m.height * 40 / 100
	}
	if topInnerH < 4 {
		topInnerH = 4
	}

	statusContent := m.renderClusterStatus(topInnerH)
	nodeContent := m.renderNodeHealth(topInnerH)

	statusPanel := panelBorder.Width(leftW).Height(topInnerH).Render(statusContent)
	nodePanel := panelBorder.Width(rightW).Height(topInnerH).Render(nodeContent)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, statusPanel, nodePanel)

	// --- Middle row: Service Matrix (full width) ---
	svcInnerH := len(allServices) + 2
	maxSvcH := m.height * 35 / 100
	if svcInnerH > maxSvcH {
		svcInnerH = maxSvcH
	}
	if svcInnerH < 4 {
		svcInnerH = 4
	}

	svcContent := m.renderServiceMatrix(svcInnerH)
	svcPanel := panelBorder.Width(fullW).Height(svcInnerH).Render(svcContent)

	// --- Bottom row: Diagnostics (left) + Events (right) ---
	topRowH := lipgloss.Height(topRow)
	svcPanelH := lipgloss.Height(svcPanel)
	bottomInnerH := m.height - topRowH - svcPanelH - 2 // -2 for bottom border
	if bottomInnerH < 3 {
		bottomInnerH = 3
	}

	diagContent := m.renderDiagnostics(bottomInnerH)
	eventContent := m.renderEvents(bottomInnerH)

	diagPanel := panelBorder.Width(leftW).Height(bottomInnerH).Render(diagContent)
	eventPanel := panelBorder.Width(rightW).Height(bottomInnerH).Render(eventContent)

	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, diagPanel, eventPanel)

	// Combine all rows
	full := lipgloss.JoinVertical(lipgloss.Left, topRow, svcPanel, bottomRow)

	// Truncate to terminal height
	lines := strings.Split(full, "\n")
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
		m.fetchCPU(),
		m.fetchEvents(),
		m.fetchDiagnostics(),
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		targets, resolve := cluster.NodeTargets(ctx, client)
		nodeCtx := talosclient.WithNodes(ctx, targets...)
		resp, err := client.C.ServiceList(nodeCtx)
		if err != nil {
			return servicesLoadedMsg{err: err}
		}

		byNode := make(map[string][]serviceRow)
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			hostname := resolve(nodeMsg.GetMetadata().GetHostname())
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		targets, resolve := cluster.NodeTargets(ctx, client)
		nodeCtx := talosclient.WithNodes(ctx, targets...)
		resp, err := client.C.Memory(nodeCtx)
		if err != nil {
			return memoryLoadedMsg{err: err}
		}

		byNode := make(map[string]memStats)
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil || nodeMsg.GetMeminfo() == nil {
				continue
			}
			hostname := resolve(nodeMsg.GetMetadata().GetHostname())
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
		for i := 0; i < 40; i++ {
			raw, err := stream.Recv()
			if err != nil {
				break
			}

			ev, err := talosclient.UnmarshalEvent(raw)
			if err != nil {
				continue
			}

			node := shortenHostname(ev.Node)
			typeName := ev.TypeURL
			if idx := strings.LastIndex(typeName, "."); idx >= 0 {
				typeName = typeName[idx+1:]
			}
			// Strip "talos/runtime/" prefix if present
			if strings.HasPrefix(typeName, "talos/runtime/") {
				typeName = typeName[len("talos/runtime/"):]
			}

			message := formatEventPayload(typeName, ev.Payload)

			events = append(events, eventRow{
				Node:    node,
				Actor:   ev.ActorID,
				Message: message,
				Time:    time.Now(),
			})
		}

		if len(events) > 20 {
			events = events[len(events)-20:]
		}

		return eventsLoadedMsg{events: events}
	}
}

// formatEventPayload extracts a human-readable message from a Talos event payload.
func formatEventPayload(typeName string, payload interface{}) string {
	switch p := payload.(type) {
	case *machineapi.ServiceStateEvent:
		action := p.GetAction().String()
		svc := p.GetService()
		msg := p.GetMessage()
		if msg != "" {
			return fmt.Sprintf("%s %s: %s", svc, action, msg)
		}
		return fmt.Sprintf("%s %s", svc, action)
	case *machineapi.AddressEvent:
		return fmt.Sprintf("AddressEvent: %s", strings.Join(p.GetAddresses(), ", "))
	case *machineapi.MachineStatusEvent:
		stage := p.GetStage().String()
		status := p.GetStatus().GetReady()
		return fmt.Sprintf("MachineStatus: %s ready=%v", stage, status)
	case *machineapi.SequenceEvent:
		seq := p.GetSequence()
		action := p.GetAction().String()
		if p.GetError() != nil {
			return fmt.Sprintf("%s %s: %s", seq, action, p.GetError().GetMessage())
		}
		return fmt.Sprintf("%s %s", seq, action)
	case *machineapi.PhaseEvent:
		return fmt.Sprintf("Phase: %s %s", p.GetPhase(), p.GetAction().String())
	case *machineapi.TaskEvent:
		return fmt.Sprintf("Task: %s %s", p.GetTask(), p.GetAction().String())
	case *machineapi.ConfigLoadErrorEvent:
		return fmt.Sprintf("ConfigLoadError: %s", p.GetError())
	case *machineapi.ConfigValidationErrorEvent:
		return fmt.Sprintf("ConfigValidationError: %s", p.GetError())
	case *machineapi.RestartEvent:
		return fmt.Sprintf("Restart: %d", p.GetCmd())
	default:
		return typeName
	}
}

func (m Model) fetchDiagnostics() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return diagnosticsLoadedMsg{diagnostics: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		diags, err := resources.ListDiagnostics(ctx, client)
		return diagnosticsLoadedMsg{diagnostics: diags, err: err}
	}
}

func (m Model) fetchCPU() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return cpuLoadedMsg{}
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

// --- Rendering helpers ---

func (m Model) renderClusterStatus(maxLines int) string {
	cpCount := 0
	workerCount := 0
	healthyCount := 0
	failedCount := 0
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
		if n.Healthy {
			healthyCount++
		}
		// Check service health
		nodeFailed := false
		if svcs, ok := m.servicesByNode[n.Hostname]; ok {
			for _, s := range svcs {
				if s.Health == "Failed" {
					nodeFailed = true
					break
				}
			}
		}
		if nodeFailed {
			failedCount++
		}
	}

	contextName := ""
	if m.client != nil {
		contextName = m.client.ContextName
	}

	title := shared.StyleHeader.Render("CLUSTER STATUS")
	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("%s  %s", shared.StyleMuted.Render("Context:"), shared.StyleValue.Render(contextName)))
	lines = append(lines, fmt.Sprintf("%s  %s",
		shared.StyleMuted.Render("Nodes:"),
		shared.StyleValue.Render(fmt.Sprintf("%d  (%d CP / %d Worker)", len(m.nodes), cpCount, workerCount))))
	lines = append(lines, fmt.Sprintf("%s  %s", shared.StyleMuted.Render("Version:"), shared.StyleValue.Render(version)))

	// Overall health
	healthStr := shared.StyleSuccess.Render(shared.StatusIcon("Running") + " All healthy")
	if failedCount > 0 {
		healthStr = shared.StyleError.Render(fmt.Sprintf("%s %d node(s) degraded", shared.StatusIcon("Failed"), failedCount))
	} else if len(m.servicesByNode) == 0 {
		healthStr = shared.StyleMuted.Render("? Awaiting data")
	}
	lines = append(lines, fmt.Sprintf("%s  %s", shared.StyleMuted.Render("Health:"), healthStr))

	// Refresh time
	if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh).Truncate(time.Second)
		lines = append(lines, fmt.Sprintf("%s  %s", shared.StyleMuted.Render("Updated:"), shared.StyleValue.Render(ago.String()+" ago")))
	}

	// Error
	if len(m.errs) > 0 {
		for _, e := range m.errs {
			lines = append(lines, shared.StyleError.Render(fmt.Sprintf("Error: %v", e)))
		}
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines[:maxLines], "\n")
}

// RenderNodeHealth renders the node health panel with memory bars.
func RenderNodeHealth(nodes []cluster.NodeInfo, servicesByNode map[string][]serviceRow, memoryByNode map[string]memStats, cpuByNode map[string]resources.CPUStats, maxLines, barWidth int) string {
	title := shared.StyleHeader.Render("NODE HEALTH")
	lines := []string{title}

	if len(nodes) == 0 {
		lines = append(lines, shared.StyleMuted.Render("No nodes found"))
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return strings.Join(lines[:maxLines], "\n")
	}

	for i, n := range nodes {
		if i+2 >= maxLines {
			break
		}
		typeStr := shared.StyleMuted.Render("Wk")
		if n.IsControlPlane() {
			typeStr = lipgloss.NewStyle().Foreground(shared.ColorPrimary).Render("CP")
		}

		// Health icon
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

		// CPU%
		cpuStr := shared.StyleMuted.Render(" -- ")
		if cs, ok := cpuByNode[n.Hostname]; ok {
			cpuStr = fmt.Sprintf("%2.0f%%", cs.UsagePercent*100)
		}

		// Memory bar
		memBar := shared.StyleMuted.Render(" N/A")
		if mem, ok := memoryByNode[n.Hostname]; ok && mem.TotalKB > 0 {
			used := mem.TotalKB - mem.AvailableKB
			pct := float64(used) / float64(mem.TotalKB)
			memBar = renderMemBar(pct, barWidth)
		}

		// Uptime
		uptimeStr := ""
		if cs, ok := cpuByNode[n.Hostname]; ok && !cs.BootTime.IsZero() {
			uptimeStr = shared.StyleMuted.Render(" " + formatUptime(time.Since(cs.BootTime)))
		}

		row := fmt.Sprintf("%-14s %s %s %s %s%s",
			shared.Truncate(shortenHostname(n.Hostname), 14),
			typeStr,
			healthStyle.Render(healthIcon),
			cpuStr,
			memBar,
			uptimeStr,
		)
		lines = append(lines, row)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m Model) renderNodeHealth(maxLines int) string {
	barW := m.width/2 - 30
	if barW < 8 {
		barW = 8
	}
	if barW > 30 {
		barW = 30
	}
	return RenderNodeHealth(m.nodes, m.servicesByNode, m.memoryByNode, m.cpuByNode, maxLines, barW)
}

// renderMemBar creates a block-character memory bar like "62% ████████░░░░"
func renderMemBar(pct float64, width int) string {
	if width < 4 {
		width = 4
	}
	pctStr := fmt.Sprintf("%2.0f%%", pct*100)
	barW := width - 4 // space for "62% "
	if barW < 1 {
		barW = 1
	}
	filled := int(pct * float64(barW))
	if filled > barW {
		filled = barW
	}
	empty := barW - filled

	barStyle := shared.StyleSuccess
	if pct > 0.8 {
		barStyle = shared.StyleError
	} else if pct > 0.6 {
		barStyle = shared.StyleWarning
	}

	bar := barStyle.Render(strings.Repeat("█", filled)) +
		shared.StyleMuted.Render(strings.Repeat("░", empty))
	return fmt.Sprintf("%s %s", pctStr, bar)
}

// RenderServiceMatrix renders the service status matrix. Exported for testing.
func RenderServiceMatrix(nodes []cluster.NodeInfo, servicesByNode map[string][]serviceRow, maxLines int) string {
	title := shared.StyleHeader.Render("SERVICE MATRIX")
	if len(nodes) == 0 {
		return title + "\n" + shared.StyleMuted.Render("No nodes")
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

	cpSet := make(map[string]bool)
	for _, s := range cpServices {
		cpSet[s] = true
	}
	workerSet := make(map[string]bool)
	for _, s := range workerServices {
		workerSet[s] = true
	}

	shortNames := make([]string, len(nodes))
	for i, n := range nodes {
		shortNames[i] = shortenHostname(n.Hostname)
	}

	nodeColWidth := 8
	header := fmt.Sprintf("%-14s", "SERVICE")
	for _, sn := range shortNames {
		header += fmt.Sprintf("%-*s", nodeColWidth, shared.Truncate(sn, nodeColWidth-1))
	}
	lines := []string{title, shared.StyleMuted.Render(header)}

	for idx, svcName := range allServices {
		if idx+3 >= maxLines {
			break
		}
		row := fmt.Sprintf("%-14s", svcName)
		for _, n := range nodes {
			expectedSet := workerSet
			if n.IsControlPlane() {
				expectedSet = cpSet
			}

			if !expectedSet[svcName] {
				row += shared.StyleMuted.Render(fmt.Sprintf("%-*s", nodeColWidth, "·"))
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

func (m Model) renderDiagnostics(maxLines int) string {
	title := shared.StyleHeader.Render("DIAGNOSTICS")
	lines := []string{title}

	if len(m.diagnostics) == 0 {
		lines = append(lines, shared.StyleSuccess.Render(shared.StatusIcon("Running")+" No issues"))
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return strings.Join(lines[:maxLines], "\n")
	}

	for i, d := range m.diagnostics {
		if i+2 >= maxLines {
			break
		}
		node := shortenHostname(d.NodeHostname)
		style := shared.StyleWarning
		icon := shared.StatusIcon("Degraded")
		if d.Severity == "error" {
			style = shared.StyleError
			icon = shared.StatusIcon("Failed")
		} else if d.Severity == "info" {
			style = shared.StyleMuted
			icon = shared.StatusIcon("Running")
		}
		line := fmt.Sprintf("%s %s %s",
			style.Render(icon),
			shared.StyleMuted.Render(fmt.Sprintf("[%-8s]", node)),
			d.Message,
		)
		lines = append(lines, line)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m Model) renderEvents(maxLines int) string {
	followTag := ""
	if m.followEvents {
		followTag = shared.StyleMuted.Render(" (following)")
	}
	title := shared.StyleHeader.Render("RECENT EVENTS") + followTag
	lines := []string{title}

	if len(m.events) == 0 {
		lines = append(lines, shared.StyleMuted.Render("No events"))
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return strings.Join(lines[:maxLines], "\n")
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
		ts := shared.StyleMuted.Render(ev.Time.Format("15:04:05"))
		nodeTag := lipgloss.NewStyle().Foreground(nodeColor).Render(fmt.Sprintf("[%-6s]", shared.Truncate(ev.Node, 6)))
		line := fmt.Sprintf("%s %s %s", ts, nodeTag, ev.Message)
		lines = append(lines, line)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m *Model) appendErr(err error) {
	// Keep at most 3 errors to avoid clutter
	if len(m.errs) >= 3 {
		m.errs = m.errs[1:]
	}
	m.errs = append(m.errs, err)
}

func (m *Model) clearErr() {
	m.errs = nil
}

// --- Utility functions ---

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

func shortenHostname(hostname string) string {
	// Try to shorten: "my-cluster-cp-1" -> "cp-1"
	parts := strings.Split(hostname, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "-")
	}
	return hostname
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

// SortNodesByHostname sorts nodes alphabetically. Exported for testing.
func SortNodesByHostname(nodes []cluster.NodeInfo) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Hostname < nodes[j].Hostname
	})
}
