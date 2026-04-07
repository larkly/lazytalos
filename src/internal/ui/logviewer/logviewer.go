// Package logviewer provides the multi-node streaming log viewer tab.
package logviewer

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"google.golang.org/grpc"
)

// Default services to show in the selector.
var defaultServices = []string{
	"apid", "containerd", "cri", "dashboard", "etcd",
	"kubelet", "machined", "syslogd", "trustd", "udevd",
}

// MaxLines is the default maximum lines in the ring buffer.
const MaxLines = 5000

// StreamKey identifies a unique node+service stream.
type StreamKey struct{ Node, Service string }

// LogLine is a single log entry.
type LogLine struct {
	Node    string
	Service string
	Text    string
	IsErr   bool
	T       time.Time
}

// activeStream holds stream handle and cancellation.
type activeStream struct {
	cancel context.CancelFunc
	stream grpc.ServerStreamingClient[common.Data]
}

// Model is the log viewer view model.
type Model struct {
	client *talos.Client

	// Selector state (left pane)
	nodes          []string
	services       []string
	activeNodes    map[string]bool
	activeServices map[string]bool
	selectorCol    int  // 0 = nodes, 1 = services
	selectorRow    int  // cursor within active column
	selectorFocus  bool // true = left pane has focus

	// Stream management
	streams map[StreamKey]activeStream

	// Log buffer
	lines    []LogLine
	maxLines int
	scrollY  int
	follow   bool

	width  int
	height int

	refreshInterval time.Duration
}

// New creates a new log viewer model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	svcs := make([]string, len(defaultServices))
	copy(svcs, defaultServices)
	return Model{
		client:          client,
		services:        svcs,
		activeNodes:     make(map[string]bool),
		activeServices:  make(map[string]bool),
		streams:         make(map[StreamKey]activeStream),
		maxLines:        MaxLines,
		follow:          true,
		selectorFocus:   true,
		refreshInterval: refreshInterval,
	}
}

// SetNodes sets the available node list (called by app when members are known).
func (m *Model) SetNodes(nodes []string) {
	m.nodes = nodes
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case shared.LogLineMsg:
		line := LogLine{
			Node:    msg.NodeID,
			Service: msg.Service,
			Text:    msg.Line,
			IsErr:   msg.IsErr,
			T:       time.Now(),
		}
		m.appendLine(line)

		// Chain next read from this stream
		sk := StreamKey{Node: msg.NodeID, Service: msg.Service}
		if as, ok := m.streams[sk]; ok {
			return m, awaitLogLine(as.stream, msg.NodeID, msg.Service)
		}

	case shared.LogStreamEndedMsg:
		sk := StreamKey{Node: msg.NodeID, Service: msg.Service}
		delete(m.streams, sk)
		m.appendLine(LogLine{
			Node:    msg.NodeID,
			Service: msg.Service,
			Text:    "--- stream ended ---",
			T:       time.Now(),
		})

	case shared.TickMsg:
		// Log viewer is stream-driven, not poll-driven. Nothing to do on tick.
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.LogFollow):
		m.follow = !m.follow

	case key.Matches(msg, shared.Keys.Tab):
		if m.selectorFocus {
			m.selectorCol = 1 - m.selectorCol
			m.selectorRow = 0
		} else {
			m.selectorFocus = true
		}

	case key.Matches(msg, shared.Keys.Back):
		if m.selectorFocus {
			m.selectorFocus = false
		}

	case key.Matches(msg, shared.Keys.Down):
		if m.selectorFocus {
			maxRow := m.selectorMaxRow()
			if m.selectorRow < maxRow-1 {
				m.selectorRow++
			}
		} else {
			m.follow = false
			m.scrollY++
			maxScroll := len(m.lines) - m.logViewHeight()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollY > maxScroll {
				m.scrollY = maxScroll
			}
		}

	case key.Matches(msg, shared.Keys.Up):
		if m.selectorFocus {
			if m.selectorRow > 0 {
				m.selectorRow--
			}
		} else {
			m.follow = false
			if m.scrollY > 0 {
				m.scrollY--
			}
		}

	case key.Matches(msg, shared.Keys.Select), key.Matches(msg, shared.Keys.Enter):
		if m.selectorFocus {
			return m.toggleSelection()
		}

	case key.Matches(msg, shared.Keys.PageDown):
		if !m.selectorFocus {
			m.follow = false
			m.scrollY += m.logViewHeight()
			maxScroll := len(m.lines) - m.logViewHeight()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollY > maxScroll {
				m.scrollY = maxScroll
			}
		}

	case key.Matches(msg, shared.Keys.PageUp):
		if !m.selectorFocus {
			m.follow = false
			m.scrollY -= m.logViewHeight()
			if m.scrollY < 0 {
				m.scrollY = 0
			}
		}
	}

	return m, nil
}

func (m Model) selectorMaxRow() int {
	if m.selectorCol == 0 {
		return len(m.nodes)
	}
	return len(m.services)
}

func (m Model) toggleSelection() (Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.selectorCol == 0 && m.selectorRow < len(m.nodes) {
		node := m.nodes[m.selectorRow]
		if m.activeNodes[node] {
			// Deactivate: cancel all streams for this node
			delete(m.activeNodes, node)
			cmds = append(cmds, m.cancelStreamsForNode(node)...)
		} else {
			m.activeNodes[node] = true
			cmds = append(cmds, m.startStreamsForNode(node)...)
		}
	} else if m.selectorCol == 1 && m.selectorRow < len(m.services) {
		svc := m.services[m.selectorRow]
		if m.activeServices[svc] {
			delete(m.activeServices, svc)
			cmds = append(cmds, m.cancelStreamsForService(svc)...)
		} else {
			m.activeServices[svc] = true
			cmds = append(cmds, m.startStreamsForService(svc)...)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) startStreamsForNode(node string) []tea.Cmd {
	var cmds []tea.Cmd
	for svc := range m.activeServices {
		cmd := m.startStream(node, svc)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func (m *Model) startStreamsForService(svc string) []tea.Cmd {
	var cmds []tea.Cmd
	for node := range m.activeNodes {
		cmd := m.startStream(node, svc)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func (m *Model) cancelStreamsForNode(node string) []tea.Cmd {
	for sk, as := range m.streams {
		if sk.Node == node {
			as.cancel()
			delete(m.streams, sk)
			m.appendLine(LogLine{
				Node:    node,
				Service: sk.Service,
				Text:    "--- stream closed ---",
				T:       time.Now(),
			})
		}
	}
	return nil
}

func (m *Model) cancelStreamsForService(svc string) []tea.Cmd {
	for sk, as := range m.streams {
		if sk.Service == svc {
			as.cancel()
			delete(m.streams, sk)
			m.appendLine(LogLine{
				Node:    sk.Node,
				Service: svc,
				Text:    "--- stream closed ---",
				T:       time.Now(),
			})
		}
	}
	return nil
}

func (m *Model) startStream(node, svc string) tea.Cmd {
	sk := StreamKey{Node: node, Service: svc}
	if _, exists := m.streams[sk]; exists {
		return nil // already streaming
	}
	if m.client == nil || m.client.C == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	nodeCtx := talosclient.WithNodes(ctx, node)

	var tailLines int32 = 50
	stream, err := m.client.C.Logs(nodeCtx, "system", common.ContainerDriver_CONTAINERD, svc, true, tailLines)
	if err != nil {
		cancel()
		m.appendLine(LogLine{
			Node:    node,
			Service: svc,
			Text:    fmt.Sprintf("--- stream error: %v ---", err),
			IsErr:   true,
			T:       time.Now(),
		})
		return nil
	}

	m.streams[sk] = activeStream{
		cancel: cancel,
		stream: stream,
	}

	return awaitLogLine(stream, node, svc)
}

// awaitLogLine returns a Cmd that blocks on the next log line from the stream.
func awaitLogLine(stream machineapi.MachineService_LogsClient, node, svc string) tea.Cmd {
	return func() tea.Msg {
		data, err := stream.Recv()
		if err != nil {
			return shared.LogStreamEndedMsg{NodeID: node, Service: svc, Err: err}
		}
		text := string(data.GetBytes())
		text = strings.TrimRight(text, "\n")
		return shared.LogLineMsg{
			NodeID:  node,
			Service: svc,
			Line:    text,
			IsErr:   false,
		}
	}
}

func (m *Model) appendLine(line LogLine) {
	m.lines = append(m.lines, line)
	if len(m.lines) > m.maxLines {
		// Drop oldest entries
		drop := len(m.lines) - m.maxLines
		m.lines = m.lines[drop:]
		m.scrollY -= drop
		if m.scrollY < 0 {
			m.scrollY = 0
		}
	}
}

func (m Model) logViewHeight() int {
	h := m.height - 1
	if h < 1 {
		return 1
	}
	return h
}

// View renders the log viewer.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	selectorWidth := m.width * 30 / 100
	if selectorWidth < 20 {
		selectorWidth = 20
	}
	logWidth := m.width - selectorWidth - 1

	selector := m.renderSelector(selectorWidth)
	logPane := m.renderLogPane(logWidth)

	// Place side by side
	selectorLines := strings.Split(selector, "\n")
	logLines := strings.Split(logPane, "\n")

	maxH := m.height
	var combined []string
	for i := 0; i < maxH; i++ {
		left := ""
		if i < len(selectorLines) {
			left = selectorLines[i]
		}
		right := ""
		if i < len(logLines) {
			right = logLines[i]
		}
		// Pad left pane
		leftW := lipgloss.Width(left)
		if leftW < selectorWidth {
			left += strings.Repeat(" ", selectorWidth-leftW)
		}
		sep := lipgloss.NewStyle().Foreground(shared.ColorMuted).Render("\u2502")
		combined = append(combined, left+sep+right)
	}

	return strings.Join(combined, "\n")
}

func (m Model) renderSelector(width int) string {
	var lines []string

	// Nodes section
	nodesHeader := "NODES"
	if m.selectorCol == 0 && m.selectorFocus {
		nodesHeader = shared.StyleHeader.Render(nodesHeader)
	} else {
		nodesHeader = shared.StyleMuted.Render(nodesHeader)
	}
	lines = append(lines, "  "+nodesHeader)

	for i, node := range m.nodes {
		check := " "
		if m.activeNodes[node] {
			check = "x"
		}
		cursor := " "
		if m.selectorCol == 0 && m.selectorRow == i && m.selectorFocus {
			cursor = ">"
		}
		shortNode := node
		if len(shortNode) > width-8 {
			shortNode = shortNode[:width-9] + "\u2026"
		}
		line := fmt.Sprintf(" %s[%s] %s", cursor, check, shortNode)
		lines = append(lines, line)
	}

	lines = append(lines, "")

	// Services section
	svcHeader := "SERVICES"
	if m.selectorCol == 1 && m.selectorFocus {
		svcHeader = shared.StyleHeader.Render(svcHeader)
	} else {
		svcHeader = shared.StyleMuted.Render(svcHeader)
	}
	lines = append(lines, "  "+svcHeader)

	for i, svc := range m.services {
		check := " "
		if m.activeServices[svc] {
			check = "x"
		}
		cursor := " "
		if m.selectorCol == 1 && m.selectorRow == i && m.selectorFocus {
			cursor = ">"
		}
		line := fmt.Sprintf(" %s[%s] %s", cursor, check, svc)
		lines = append(lines, line)
	}

	// Pad to height
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:m.height], "\n")
}

func (m Model) renderLogPane(width int) string {
	viewH := m.logViewHeight()
	var lines []string

	if len(m.lines) == 0 {
		lines = append(lines, shared.StyleMuted.Render(" Select nodes and services to stream logs"))
		for len(lines) < viewH {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	startIdx := 0
	if m.follow {
		startIdx = len(m.lines) - viewH
	} else {
		startIdx = m.scrollY
	}
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(m.lines) && len(lines) < viewH; i++ {
		line := m.lines[i]
		nodeColor := nodeColorFor(line.Node)
		nodeTag := lipgloss.NewStyle().Foreground(nodeColor).Render(fmt.Sprintf("[%-8s]", truncate(line.Node, 8)))
		svcTag := shared.StyleMuted.Render(fmt.Sprintf("[%-10s]", truncate(line.Service, 10)))

		text := line.Text
		maxTextWidth := width - 22
		if maxTextWidth > 0 && len(text) > maxTextWidth {
			text = text[:maxTextWidth]
		}

		rendered := fmt.Sprintf(" %s%s %s", nodeTag, svcTag, text)
		lines = append(lines, rendered)
	}

	// Pad to height
	for len(lines) < viewH {
		lines = append(lines, "")
	}

	// Status line
	followStr := ""
	if m.follow {
		followStr = shared.StyleSuccess.Render(" FOLLOW")
	} else {
		followStr = shared.StyleMuted.Render(" PAUSED")
	}
	streamCount := len(m.streams)
	status := fmt.Sprintf(" %s  %d streams  %d lines", followStr, streamCount, len(m.lines))
	if len(lines) > 0 {
		lines[len(lines)-1] = status
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
	if m.selectorFocus {
		return "tab:switch col  space/enter:toggle  esc:log pane  F:follow"
	}
	return "tab:selector  F:follow  ↑↓/pgup/pgdn:scroll"
}

// ForceRefresh triggers an immediate data refresh.
// Log viewer is stream-driven; nothing to refresh on demand.
func (m Model) ForceRefresh() tea.Cmd {
	return nil
}

// --- Utilities ---

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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "\u2026"
}

// AppendLine is exported for testing.
func (m *Model) AppendLine(line LogLine) {
	m.appendLine(line)
}

// Lines returns the log buffer for testing.
func (m Model) Lines() []LogLine {
	return m.lines
}

// Streams returns the active stream map for testing.
func (m Model) Streams() map[StreamKey]activeStream {
	return m.streams
}
