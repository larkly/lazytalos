// Package storage provides the Storage tab view with Devices and Volumes sub-tabs.
package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/resources"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

const (
	subTabDevices = 0
	subTabVolumes = 1
	numSubTabs    = 2
)

var subTabNames = [numSubTabs]string{"Devices", "Volumes"}

// storageLoadedMsg is the internal message returned after fetching all storage data.
type storageLoadedMsg struct {
	devices  []resources.BlockDevice
	vols     []resources.DiscoveredVolume
	statuses []resources.VolumeStatus
	err      error
}

// Model is the storage tab view model.
type Model struct {
	client          *talos.Client
	subTab          int
	devices         []resources.BlockDevice
	discoveredVols  []resources.DiscoveredVolume
	volStatuses     []resources.VolumeStatus
	loading         bool
	err             error
	cursor          int
	scrollY         int
	width, height   int
	refreshInterval time.Duration
}

// New creates a new storage model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
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
		switch {
		case key.Matches(msg, shared.Keys.SubTabNext):
			m.subTab = (m.subTab + 1) % numSubTabs
			m.cursor = 0
			m.scrollY = 0
		case key.Matches(msg, shared.Keys.SubTabPrev):
			m.subTab = (m.subTab - 1 + numSubTabs) % numSubTabs
			m.cursor = 0
			m.scrollY = 0
		case key.Matches(msg, shared.Keys.Up):
			if m.subTab == subTabDevices {
				if m.cursor > 0 {
					m.cursor--
				}
			} else {
				if m.scrollY > 0 {
					m.scrollY--
				}
			}
		case key.Matches(msg, shared.Keys.Down):
			if m.subTab == subTabDevices {
				if m.cursor < len(m.devices)-1 {
					m.cursor++
				}
			} else {
				m.scrollY++
			}
		}

	case shared.TickMsg:
		return m, m.ForceRefresh()

	case storageLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.devices = msg.devices
			m.discoveredVols = msg.vols
			m.volStatuses = msg.statuses
			m.err = nil
		}
		m.loading = false
	}

	return m, nil
}

// View renders the storage tab.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Sub-tab bar
	b.WriteString(m.renderSubTabBar())
	b.WriteByte('\n')

	// Body
	if m.loading {
		b.WriteString(shared.StyleMuted.Render("  Loading storage data..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	bodyLines := m.height - 2 // subtract sub-tab bar + newline
	if bodyLines < 1 {
		bodyLines = 1
	}

	switch m.subTab {
	case subTabDevices:
		b.WriteString(m.renderDevices(bodyLines))
	case subTabVolumes:
		b.WriteString(m.renderVolumes(bodyLines))
	}

	// Truncate to height
	content := b.String()
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
	return "</>:sub-tab  ↑↓:navigate  ctrl+r:refresh"
}

// ForceRefresh triggers an immediate data reload.
func (m Model) ForceRefresh() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil {
			return storageLoadedMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		devices, err := resources.ListBlockDevices(ctx, client)
		if err != nil {
			return storageLoadedMsg{err: err}
		}

		vols, err := resources.ListDiscoveredVolumes(ctx, client)
		if err != nil {
			return storageLoadedMsg{err: err}
		}

		statuses, err := resources.ListVolumeStatuses(ctx, client)
		if err != nil {
			return storageLoadedMsg{err: err}
		}

		return storageLoadedMsg{
			devices:  devices,
			vols:     vols,
			statuses: statuses,
		}
	}
}

// --- Rendering helpers ---

func (m Model) renderSubTabBar() string {
	var parts []string
	for i, name := range subTabNames {
		label := " " + name + " "
		if i == m.subTab {
			parts = append(parts, shared.StyleTabActive.Render(label))
		} else {
			parts = append(parts, shared.StyleTabInactive.Render(label))
		}
	}
	return strings.Join(parts, "")
}

func (m Model) renderDevices(maxLines int) string {
	header := fmt.Sprintf("  %-22s %-6s %-6s %-12s %s",
		"NODE", "NAME", "TYPE", "SIZE", "BUS PATH")
	lines := []string{shared.StyleHeader.Render(header)}

	for i, d := range m.devices {
		if i >= maxLines {
			break
		}
		sizeStr := formatSize(d.Size)
		row := fmt.Sprintf("  %-22s %-6s %-6s %-12s %s",
			shared.Truncate(d.NodeHostname, 22),
			shared.Truncate(d.Name, 6),
			shared.Truncate(d.DevType, 6),
			sizeStr,
			d.BusPath,
		)
		if i == m.cursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}

	if len(m.devices) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No block devices found"))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderVolumes(maxLines int) string {
	var lines []string

	// Discovered volumes section
	dvHeader := fmt.Sprintf("  %-22s %-8s %-8s %-8s %-22s %s",
		"NODE", "NAME", "FSTYPE", "LABEL", "UUID", "SIZE")
	lines = append(lines, shared.StyleHeader.Render("  Discovered Volumes:"))
	lines = append(lines, shared.StyleHeader.Render(dvHeader))

	for _, v := range m.discoveredVols {
		sizeStr := formatSize(v.Size)
		row := fmt.Sprintf("  %-22s %-8s %-8s %-8s %-22s %s",
			shared.Truncate(v.NodeHostname, 22),
			shared.Truncate(v.Name, 8),
			shared.Truncate(v.FSType, 8),
			shared.Truncate(v.Label, 8),
			shared.Truncate(v.UUID, 22),
			sizeStr,
		)
		lines = append(lines, row)
	}
	if len(m.discoveredVols) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No discovered volumes"))
	}

	// Separator
	lines = append(lines, "")

	// Volume statuses section
	vsHeader := fmt.Sprintf("  %-22s %-14s %-10s %s",
		"NODE", "NAME", "PHASE", "MOUNT")
	lines = append(lines, shared.StyleHeader.Render("  Volume Statuses:"))
	lines = append(lines, shared.StyleHeader.Render(vsHeader))

	for _, s := range m.volStatuses {
		row := fmt.Sprintf("  %-22s %-14s %-10s %s",
			shared.Truncate(s.NodeHostname, 22),
			shared.Truncate(s.Name, 14),
			shared.Truncate(s.Phase, 10),
			s.MountSpec,
		)
		lines = append(lines, row)
	}
	if len(m.volStatuses) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No volume statuses"))
	}

	// Apply scroll
	start := m.scrollY
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

// --- Utility helpers ---

// formatSize converts bytes to a human-readable string (GiB or MiB).
func formatSize(bytes uint64) string {
	const gib = 1073741824.0
	const mib = 1048576.0
	if bytes == 0 {
		return "0"
	}
	if float64(bytes) >= gib {
		return fmt.Sprintf("%.1f GiB", float64(bytes)/gib)
	}
	return fmt.Sprintf("%.1f MiB", float64(bytes)/mib)
}

