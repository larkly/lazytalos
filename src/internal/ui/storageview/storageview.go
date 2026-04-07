// Package storageview provides the Storage tab with Devices and Volumes sub-tabs.
package storageview

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type subTab int

const (
	subTabDevices subTab = iota
	subTabVolumes
	numSubTabs
)

var subTabNames = [numSubTabs]string{"Devices", "Volumes"}

type deviceRow struct {
	Node       string
	DevPath    string
	PrettySize string
	Model      string
	Serial     string
	Readonly   bool
}

type volumeRow struct {
	Node          string
	ID            string
	Phase         string
	PrettySize    string
	Location      string
	MountLocation string
}

// Internal messages.
type devicesLoadedMsg struct {
	rows []deviceRow
	err  error
}

type volumesLoadedMsg struct {
	rows []volumeRow
	err  error
}

// Model is the storage view model.
type Model struct {
	client    *talos.Client
	activeTab subTab

	devices []deviceRow
	volumes []volumeRow

	cursor    int
	scrollOff int
	filter    string
	filterActive bool

	focusLeft bool

	loading         bool
	err             error
	width           int
	height          int
	refreshInterval time.Duration
}

// New creates a new storage view model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		loading:         true,
		focusLeft:       true,
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
		return m.updateNormal(msg)

	case shared.TickMsg:
		return m, m.fetchActive()

	case devicesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.devices = msg.rows
			m.err = nil
		}
		m.loading = false

	case volumesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.volumes = msg.rows
			m.err = nil
		}
		m.loading = false
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Tab):
		m.activeTab = (m.activeTab + 1) % numSubTabs
		m.cursor = 0
		m.scrollOff = 0
		m.filter = ""
		return m, m.fetchActive()

	case key.Matches(msg, shared.Keys.ShiftTab):
		m.activeTab = (m.activeTab + numSubTabs - 1) % numSubTabs
		m.cursor = 0
		m.scrollOff = 0
		m.filter = ""
		return m, m.fetchActive()

	case key.Matches(msg, shared.Keys.Down):
		maxIdx := m.activeRowCount() - 1
		if m.cursor < maxIdx {
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
		maxIdx := m.activeRowCount() - 1
		if m.cursor > maxIdx {
			m.cursor = maxIdx
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

	case key.Matches(msg, shared.Keys.Filter):
		m.filterActive = true

	case key.Matches(msg, shared.Keys.Left):
		m.focusLeft = true

	case key.Matches(msg, shared.Keys.Right):
		m.focusLeft = false

	case key.Matches(msg, shared.Keys.Back):
		if m.filter != "" {
			m.filter = ""
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
	m.cursor = 0
	m.scrollOff = 0
	return m, nil
}

// View renders the storage view.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	leftWidth := m.width / 5
	if leftWidth < 12 {
		leftWidth = 12
	}
	rightWidth := m.width - leftWidth - 1 // 1 for separator

	leftPane := m.renderLeftPane(leftWidth, m.height)
	rightPane := m.renderRightPane(rightWidth, m.height)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (m Model) renderLeftPane(w, h int) string {
	var lines []string
	for i := 0; i < int(numSubTabs); i++ {
		name := subTabNames[i]
		marker := "  "
		if subTab(i) == m.activeTab {
			marker = shared.StyleHeader.Render("\u25b8 ")
			name = shared.StyleHeader.Render(name)
		} else {
			name = shared.StyleMuted.Render(name)
		}
		lines = append(lines, marker+name)
	}

	// Pad to fill height.
	for len(lines) < h {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(w).Render(content)
}

func (m Model) renderRightPane(w, h int) string {
	if m.loading && m.activeRowCount() == 0 {
		return shared.StyleMuted.Render("  Loading...")
	}

	var lines []string

	// Filter row.
	if m.filterActive {
		lines = append(lines, shared.StyleWarning.Render(fmt.Sprintf("  Filter: %s_", m.filter)))
	} else if m.filter != "" {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Filter: %s", m.filter)))
	}

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	switch m.activeTab {
	case subTabDevices:
		lines = append(lines, m.renderDeviceTable(w)...)
	case subTabVolumes:
		lines = append(lines, m.renderVolumeTable(w)...)
	}

	// Pad to fill height.
	for len(lines) < h {
		lines = append(lines, "")
	}

	return strings.Join(lines[:h], "\n")
}

func (m Model) renderDeviceTable(w int) []string {
	header := fmt.Sprintf("  %-20s %-12s %-10s %-20s %-16s %-10s",
		"NODE", "DEVICE", "SIZE", "MODEL", "SERIAL", "READ-ONLY")
	lines := []string{shared.StyleHeader.Render(header)}

	rows := m.filteredDevices()
	visible := m.visibleRows()
	endIdx := m.scrollOff + visible
	if endIdx > len(rows) {
		endIdx = len(rows)
	}

	for i := m.scrollOff; i < endIdx; i++ {
		r := rows[i]
		cursor := " "
		isCursor := i == m.cursor

		if isCursor {
			cursor = ">"
		}

		readOnly := "no"
		if r.Readonly {
			readOnly = "yes"
		}

		row := fmt.Sprintf("%s %-20s %-12s %-10s %-20s %-16s %-10s",
			cursor,
			truncate(r.Node, 20),
			truncate(r.DevPath, 12),
			truncate(r.PrettySize, 10),
			truncate(r.Model, 20),
			truncate(r.Serial, 16),
			readOnly,
		)

		if isCursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}

	return lines
}

func (m Model) renderVolumeTable(w int) []string {
	header := fmt.Sprintf("  %-20s %-24s %-12s %-10s %-20s %-20s",
		"NODE", "ID", "PHASE", "SIZE", "LOCATION", "MOUNT")
	lines := []string{shared.StyleHeader.Render(header)}

	rows := m.filteredVolumes()
	visible := m.visibleRows()
	endIdx := m.scrollOff + visible
	if endIdx > len(rows) {
		endIdx = len(rows)
	}

	for i := m.scrollOff; i < endIdx; i++ {
		r := rows[i]
		cursor := " "
		isCursor := i == m.cursor

		if isCursor {
			cursor = ">"
		}

		row := fmt.Sprintf("%s %-20s %-24s %-12s %-10s %-20s %-20s",
			cursor,
			truncate(r.Node, 20),
			truncate(r.ID, 24),
			truncate(r.Phase, 12),
			truncate(r.PrettySize, 10),
			truncate(r.Location, 20),
			truncate(r.MountLocation, 20),
		)

		if isCursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}

	return lines
}

func (m Model) filteredDevices() []deviceRow {
	if m.filter == "" {
		return m.devices
	}
	lower := strings.ToLower(m.filter)
	var out []deviceRow
	for _, r := range m.devices {
		if strings.Contains(strings.ToLower(r.Node), lower) ||
			strings.Contains(strings.ToLower(r.DevPath), lower) ||
			strings.Contains(strings.ToLower(r.Model), lower) ||
			strings.Contains(strings.ToLower(r.Serial), lower) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) filteredVolumes() []volumeRow {
	if m.filter == "" {
		return m.volumes
	}
	lower := strings.ToLower(m.filter)
	var out []volumeRow
	for _, r := range m.volumes {
		if strings.Contains(strings.ToLower(r.Node), lower) ||
			strings.Contains(strings.ToLower(r.ID), lower) ||
			strings.Contains(strings.ToLower(r.Phase), lower) ||
			strings.Contains(strings.ToLower(r.Location), lower) ||
			strings.Contains(strings.ToLower(r.MountLocation), lower) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) activeRowCount() int {
	switch m.activeTab {
	case subTabDevices:
		return len(m.filteredDevices())
	case subTabVolumes:
		return len(m.filteredVolumes())
	}
	return 0
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
	v := m.height - 3
	if v < 1 {
		return 1
	}
	return v
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchActive()
}

// Hints returns status bar hint text.
func (m Model) Hints() string {
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	return "tab:sub-tab  /:filter  arrows:navigate"
}

func (m Model) fetchActive() tea.Cmd {
	switch m.activeTab {
	case subTabDevices:
		return m.fetchDevices()
	case subTabVolumes:
		return m.fetchVolumes()
	}
	return nil
}

func (m Model) fetchDevices() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return devicesLoadedMsg{rows: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get cluster members first to know which nodes to query.
		members, err := cluster.GetMembers(ctx, client)
		if err != nil {
			return devicesLoadedMsg{err: err}
		}

		var rows []deviceRow

		for _, node := range members {
			addr := nodeAddress(node)
			if addr == "" {
				continue
			}

			nodeCtx := talosclient.WithNode(ctx, addr)

			meta := resource.NewMetadata(
				blockres.NamespaceName,
				blockres.DiskType,
				"",
				resource.VersionUndefined,
			)

			list, listErr := client.C.COSI.List(nodeCtx, meta)
			if listErr != nil {
				shared.Debugf("[storage] COSI List disks error for %s: %v", node.Hostname, listErr)
				continue
			}

			for _, item := range list.Items {
				disk, ok := item.(*blockres.Disk)
				if !ok {
					continue
				}
				spec := disk.TypedSpec()
				rows = append(rows, deviceRow{
					Node:       node.Hostname,
					DevPath:    spec.DevPath,
					PrettySize: spec.PrettySize,
					Model:      spec.Model,
					Serial:     spec.Serial,
					Readonly:   spec.Readonly,
				})
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Node != rows[j].Node {
				return rows[i].Node < rows[j].Node
			}
			return rows[i].DevPath < rows[j].DevPath
		})

		return devicesLoadedMsg{rows: rows}
	}
}

func (m Model) fetchVolumes() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return volumesLoadedMsg{rows: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		members, err := cluster.GetMembers(ctx, client)
		if err != nil {
			return volumesLoadedMsg{err: err}
		}

		var rows []volumeRow

		for _, node := range members {
			addr := nodeAddress(node)
			if addr == "" {
				continue
			}

			nodeCtx := talosclient.WithNode(ctx, addr)

			meta := resource.NewMetadata(
				blockres.NamespaceName,
				blockres.VolumeStatusType,
				"",
				resource.VersionUndefined,
			)

			list, listErr := client.C.COSI.List(nodeCtx, meta)
			if listErr != nil {
				shared.Debugf("[storage] COSI List volumes error for %s: %v", node.Hostname, listErr)
				continue
			}

			for _, item := range list.Items {
				vol, ok := item.(*blockres.VolumeStatus)
				if !ok {
					continue
				}
				spec := vol.TypedSpec()
				rows = append(rows, volumeRow{
					Node:          node.Hostname,
					ID:            vol.Metadata().ID(),
					Phase:         spec.Phase.String(),
					PrettySize:    spec.PrettySize,
					Location:      spec.Location,
					MountLocation: spec.MountLocation,
				})
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Node != rows[j].Node {
				return rows[i].Node < rows[j].Node
			}
			return rows[i].ID < rows[j].ID
		})

		return volumesLoadedMsg{rows: rows}
	}
}

// nodeAddress returns the first address of a cluster member node.
func nodeAddress(node cluster.NodeInfo) string {
	if len(node.Addresses) > 0 {
		return node.Addresses[0]
	}
	return ""
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
