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
	servicesByNode map[string][]shared.ServiceRow
	err            error
}

type memoryLoadedMsg struct {
	memoryByNode map[string]shared.MemStats
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
	servicesByNode  map[string][]shared.ServiceRow
	memoryByNode    map[string]shared.MemStats
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
		servicesByNode:  make(map[string][]shared.ServiceRow),
		memoryByNode:    make(map[string]shared.MemStats),
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
		m.errs = nil // clear errors each refresh cycle
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

	// --- Top row: Cluster Status (left) + Cluster Nodes dot matrix (right) ---
	// Dot matrix: title + ceil(nodes/dotsPerRow) rows + blank + legend = compact
	dotsPerRow := (rightW - 6) / 2
	if dotsPerRow < 4 {
		dotsPerRow = 4
	}
	dotRows := (len(m.nodes) + dotsPerRow - 1) / dotsPerRow
	topInnerH := dotRows + 4 // title + dot rows + blank + legend
	if topInnerH < 6 {
		topInnerH = 6
	}
	if topInnerH > m.height*35/100 {
		topInnerH = m.height * 35 / 100
	}

	statusContent := m.renderClusterStatus(topInnerH)
	nodeContent := m.renderNodeDotMatrix(topInnerH)

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
	bottomInnerH := m.height - topRowH - svcPanelH - 3 // -2 for bottom border, -1 for top blank line
	if bottomInnerH < 3 {
		bottomInnerH = 3
	}

	diagContent := m.renderDiagnostics(bottomInnerH)
	eventContent := m.renderEvents(bottomInnerH)

	diagPanel := panelBorder.Width(leftW).Height(bottomInnerH).Render(diagContent)
	eventPanel := panelBorder.Width(rightW).Height(bottomInnerH).Render(eventContent)

	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, diagPanel, eventPanel)

	// Combine all rows with a blank first line for the version overlay
	full := "\n" + lipgloss.JoinVertical(lipgloss.Left, topRow, svcPanel, bottomRow)

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
			return servicesLoadedMsg{servicesByNode: make(map[string][]shared.ServiceRow)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		targets, resolve := cluster.NodeTargets(ctx, client)
		nodeCtx := talosclient.WithNodes(ctx, targets...)
		resp, err := client.C.ServiceList(nodeCtx)

		byNode := make(map[string][]shared.ServiceRow)
		if resp == nil {
			return servicesLoadedMsg{servicesByNode: byNode, err: err}
		}
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			hostname := resolve(nodeMsg.GetMetadata().GetHostname())
			if hostname == "" {
				continue
			}
			var svcs []shared.ServiceRow
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
				svcs = append(svcs, shared.ServiceRow{
					ServiceID: svc.GetId(),
					State:     svc.GetState(),
					Health:    health,
				})
			}
			byNode[hostname] = svcs
		}
		return servicesLoadedMsg{servicesByNode: byNode, err: err}
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

func (m Model) fetchEvents() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return eventsLoadedMsg{events: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Build address→hostname resolver for event node attribution
		_, resolve := cluster.NodeTargets(ctx, client)

		stream, err := client.C.Events(ctx, talosclient.WithTailEvents(20))
		if err != nil {
			return eventsLoadedMsg{err: err}
		}

		var events []eventRow
		var lastMsg string
		for i := 0; i < 40; i++ {
			raw, err := stream.Recv()
			if err != nil {
				break
			}

			ev, err := talosclient.UnmarshalEvent(raw)
			if err != nil {
				continue
			}

			// Resolve node: try metadata hostname, then resolve IP→hostname
			node := ev.Node
			if resolved := resolve(node); resolved != node {
				node = resolved
			}
			node = shared.ShortenHostname(node)

			typeName := ev.TypeURL
			if idx := strings.LastIndex(typeName, "."); idx >= 0 {
				typeName = typeName[idx+1:]
			}
			if strings.HasPrefix(typeName, "talos/runtime/") {
				typeName = typeName[len("talos/runtime/"):]
			}

			message := formatEventPayload(typeName, ev.Payload)

			// Deduplicate consecutive identical events
			key := node + "|" + message
			if key == lastMsg {
				continue
			}
			lastMsg = key

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
	failedCount := 0
	unreachableCount := 0
	version := ""
	hasAnyData := len(m.servicesByNode) > 0
	for _, n := range m.nodes {
		if n.IsControlPlane() {
			cpCount++
		} else {
			workerCount++
		}
		if version == "" && n.TalosVersion != "" {
			version = n.TalosVersion
		}
		svcs, hasSvcs := m.servicesByNode[n.Hostname]
		if hasSvcs {
			for _, s := range svcs {
				if s.Health == "Failed" {
					failedCount++
					break
				}
			}
		} else if hasAnyData {
			unreachableCount++
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
	var healthStr string
	switch {
	case failedCount > 0 && unreachableCount > 0:
		healthStr = shared.StyleError.Render(fmt.Sprintf("%s %d degraded, %d unreachable", shared.StatusIcon("Failed"), failedCount, unreachableCount))
	case failedCount > 0:
		healthStr = shared.StyleError.Render(fmt.Sprintf("%s %d node(s) degraded", shared.StatusIcon("Failed"), failedCount))
	case unreachableCount > 0:
		healthStr = shared.StyleWarning.Render(fmt.Sprintf("%s %d node(s) unreachable", shared.StatusIcon("Stopped"), unreachableCount))
	case !hasAnyData:
		healthStr = shared.StyleMuted.Render("? Awaiting data")
	default:
		healthStr = shared.StyleSuccess.Render(shared.StatusIcon("Running") + " All healthy")
	}
	lines = append(lines, fmt.Sprintf("%s  %s", shared.StyleMuted.Render("Health:"), healthStr))

	// Refresh time
	if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh).Truncate(time.Second)
		lines = append(lines, fmt.Sprintf("%s  %s", shared.StyleMuted.Render("Updated:"), shared.StyleValue.Render(ago.String()+" ago")))
	}

	// Errors — summarize to avoid clutter from transient node failures
	if len(m.errs) > 0 {
		if len(m.errs) == 1 {
			lines = append(lines, shared.StyleWarning.Render(shared.Truncate(fmt.Sprintf("! %v", m.errs[0]), 60)))
		} else {
			lines = append(lines, shared.StyleWarning.Render(fmt.Sprintf("! %d errors (node(s) may be unreachable)", len(m.errs))))
		}
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines[:maxLines], "\n")
}

// RenderNodeDotMatrix renders a compact dot-matrix cluster overview.
// Each node is a single colored dot indicating its health state.
func RenderNodeDotMatrix(nodes []cluster.NodeInfo, servicesByNode map[string][]shared.ServiceRow, memoryByNode map[string]shared.MemStats, cpuByNode map[string]resources.CPUStats, maxLines, panelWidth int) string {
	title := shared.StyleHeader.Render("CLUSTER NODES") +
		shared.StyleMuted.Render(fmt.Sprintf(" (%d)", len(nodes)))
	lines := []string{title}

	if len(nodes) == 0 {
		lines = append(lines, shared.StyleMuted.Render("No nodes found"))
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return strings.Join(lines[:maxLines], "\n")
	}

	hasAnyData := len(servicesByNode) > 0

	// Build dot string with wrapping
	dotsPerRow := (panelWidth - 4) / 2 // each dot is "● " = 2 chars
	if dotsPerRow < 4 {
		dotsPerRow = 4
	}

	var rowDots []string
	for i, n := range nodes {
		icon := "●"
		style := shared.StyleSuccess

		svcs, hasSvcs := servicesByNode[n.Hostname]
		if !hasSvcs && hasAnyData {
			// Unreachable
			icon = "○"
			style = shared.StyleMuted
		} else if hasSvcs {
			// Check for failed services
			hasFailed := false
			for _, s := range svcs {
				if s.Health == "Failed" {
					hasFailed = true
					break
				}
			}
			if hasFailed {
				style = shared.StyleError
			} else {
				// Check resource warnings
				mem, hasMem := memoryByNode[n.Hostname]
				cpu, hasCPU := cpuByNode[n.Hostname]
				memPct := 0.0
				if hasMem && mem.TotalKB > 0 {
					memPct = float64(mem.TotalKB-mem.AvailableKB) / float64(mem.TotalKB)
				}
				if memPct > shared.MemCriticalPct {
					style = shared.StyleError
				} else if (hasCPU && cpu.UsagePercent > shared.CPUWarningPct) || memPct > shared.MemWarningPct {
					style = shared.StyleWarning
				}
			}
		}

		rowDots = append(rowDots, style.Render(icon))

		if (i+1)%dotsPerRow == 0 || i == len(nodes)-1 {
			lines = append(lines, strings.Join(rowDots, " "))
			rowDots = nil
		}
	}

	// Legend
	lines = append(lines, "")
	legend := shared.StyleSuccess.Render("●") + shared.StyleMuted.Render(" ready  ") +
		shared.StyleWarning.Render("●") + shared.StyleMuted.Render(" warn  ") +
		shared.StyleError.Render("●") + shared.StyleMuted.Render(" error  ") +
		shared.StyleMuted.Render("○ offline")
	lines = append(lines, legend)

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m Model) renderNodeDotMatrix(maxLines int) string {
	return RenderNodeDotMatrix(m.nodes, m.servicesByNode, m.memoryByNode, m.cpuByNode, maxLines, m.width/2)
}

// RenderServiceMatrix renders the service status matrix. Exported for testing.
func RenderServiceMatrix(nodes []cluster.NodeInfo, servicesByNode map[string][]shared.ServiceRow, maxLines int) string {
	title := shared.StyleHeader.Render("SERVICE MATRIX")
	if len(nodes) == 0 {
		return title + "\n" + shared.StyleMuted.Render("No nodes")
	}

	// Build service lookup per node
	svcByNodeAndID := make(map[string]map[string]shared.ServiceRow)
	for hostname, svcs := range servicesByNode {
		svcMap := make(map[string]shared.ServiceRow)
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
		shortNames[i] = shared.ShortenHostname(n.Hostname)
	}

	nodeColWidth := 8
	header := fmt.Sprintf("%-14s", "SERVICE")
	for _, sn := range shortNames {
		header += fmt.Sprintf("%-*s", nodeColWidth, shared.Truncate(sn, nodeColWidth-1))
	}
	lines := []string{title, shared.StyleMuted.Render(header)}

	hasAnyData := len(servicesByNode) > 0

	for idx, svcName := range allServices {
		if idx+3 >= maxLines {
			break
		}
		row := fmt.Sprintf("%-14s", svcName)
		for _, n := range nodes {
			// Node completely unreachable — show ○ for all services
			if _, nodeHasData := svcByNodeAndID[n.Hostname]; !nodeHasData && hasAnyData {
				row += shared.StyleWarning.Render(fmt.Sprintf("%-*s", nodeColWidth, "○"))
				continue
			}

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
				icon := "●"
				style := shared.StyleSuccess
				if svc.State != "Running" || svc.Health == "Failed" {
					icon = "✘"
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
		node := shared.ShortenHostname(d.NodeHostname)
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
	// Deduplicate by error message prefix (same root cause)
	msg := err.Error()
	for _, existing := range m.errs {
		if existing.Error() == msg {
			return
		}
	}
	if len(m.errs) >= 5 {
		m.errs = m.errs[1:]
	}
	m.errs = append(m.errs, err)
}

func (m *Model) clearErr() {
	m.errs = nil
}

// --- Utility functions ---

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
