// Package network provides the Network tab view with Addresses, Routes, and DNS sub-tabs.
package network

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

// Sub-tab indices.
const (
	subTabAddresses = 0
	subTabRoutes    = 1
	subTabDNS       = 2
	subTabCount     = 3
)

var subTabNames = [subTabCount]string{"Addresses", "Routes", "DNS"}

// networkLoadedMsg carries the result of a full network data fetch.
type networkLoadedMsg struct {
	addresses    []resources.AddressStatus
	routes       []resources.RouteStatus
	dnsUpstreams []resources.DNSUpstream
	err          error
}

// Model is the Network tab view model.
type Model struct {
	client          *talos.Client
	subTab          int
	addresses       []resources.AddressStatus
	routes          []resources.RouteStatus
	dnsUpstreams    []resources.DNSUpstream
	loading         bool
	err             error
	cursor          int
	scrollY         int
	width, height   int
	refreshInterval time.Duration
}

// New creates a new network model.
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
			m.subTab = (m.subTab + 1) % subTabCount
			m.cursor = 0
			m.scrollY = 0
		case key.Matches(msg, shared.Keys.SubTabPrev):
			m.subTab = (m.subTab + subTabCount - 1) % subTabCount
			m.cursor = 0
			m.scrollY = 0
		case key.Matches(msg, shared.Keys.Down):
			max := m.currentRowCount() - 1
			if m.cursor < max {
				m.cursor++
			}
		case key.Matches(msg, shared.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		// Left/Right: do NOT consume — let them bubble up for main tab switching.
		}

	case shared.TickMsg:
		return m, m.ForceRefresh()

	case networkLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.addresses = msg.addresses
			m.routes = msg.routes
			m.dnsUpstreams = msg.dnsUpstreams
			m.err = nil
		}
		m.loading = false
	}

	return m, nil
}

// View renders the network tab.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.loading && m.currentRowCount() == 0 {
		return shared.StyleMuted.Render("  Loading network data...")
	}

	var lines []string

	// Sub-tab bar
	lines = append(lines, m.renderSubTabBar())

	// Error
	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	// Content
	contentHeight := m.height - 2 // sub-tab bar + possible padding
	if m.err != nil {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	switch m.subTab {
	case subTabAddresses:
		lines = append(lines, m.renderAddresses(contentHeight))
	case subTabRoutes:
		lines = append(lines, m.renderRoutes(contentHeight))
	case subTabDNS:
		lines = append(lines, m.renderDNS(contentHeight))
	}

	content := strings.Join(lines, "\n")

	// Truncate to height
	allLines := strings.Split(content, "\n")
	if len(allLines) > m.height {
		allLines = allLines[:m.height]
	}
	return strings.Join(allLines, "\n")
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Hints returns status bar hint text.
func (m Model) Hints() string {
	return "[/:prev sub-tab  ]:next sub-tab  ↑↓:scroll  ctrl+r:refresh"
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return tea.Batch(
		m.fetchAddresses(),
		m.fetchRoutes(),
		m.fetchDNS(),
	)
}

// --- Data fetching ---

func (m Model) fetchAddresses() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return networkLoadedMsg{addresses: nil, routes: nil, dnsUpstreams: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		addrs, err := resources.ListAddresses(ctx, client)
		if err != nil {
			return networkLoadedMsg{err: err}
		}
		return networkLoadedMsg{addresses: addrs}
	}
}

func (m Model) fetchRoutes() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return networkLoadedMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		routes, err := resources.ListRoutes(ctx, client)
		if err != nil {
			return networkLoadedMsg{err: err}
		}
		return networkLoadedMsg{routes: routes}
	}
}

func (m Model) fetchDNS() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return networkLoadedMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dns, err := resources.ListDNSUpstreams(ctx, client)
		if err != nil {
			return networkLoadedMsg{err: err}
		}
		return networkLoadedMsg{dnsUpstreams: dns}
	}
}

// --- Rendering helpers ---

func (m Model) renderSubTabBar() string {
	var parts []string
	for i, name := range subTabNames {
		label := " " + name + " "
		if i == m.subTab {
			parts = append(parts, shared.StyleSelected.Render(label))
		} else {
			parts = append(parts, shared.StyleMuted.Render(label))
		}
	}
	return strings.Join(parts, "")
}

func (m Model) renderAddresses(maxLines int) string {
	header := fmt.Sprintf("  %-24s %-12s %-22s %-10s %-12s",
		"NODE", "INTERFACE", "ADDRESS", "SCOPE", "FLAGS")
	lines := []string{shared.StyleHeader.Render(header)}

	if len(m.addresses) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No addresses found"))
		return strings.Join(lines, "\n")
	}

	visibleStart, visibleEnd := m.visibleRange(len(m.addresses), maxLines-1)
	for i := visibleStart; i < visibleEnd; i++ {
		a := m.addresses[i]
		row := fmt.Sprintf("  %-24s %-12s %-22s %-10s %-12s",
			truncate(a.NodeHostname, 24),
			truncate(a.Interface, 12),
			truncate(a.Address, 22),
			truncate(a.Scope, 10),
			truncate(a.Flags, 12),
		)
		if i == m.cursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderRoutes(maxLines int) string {
	header := fmt.Sprintf("  %-24s %-20s %-16s %-12s %-8s",
		"NODE", "DESTINATION", "GATEWAY", "INTERFACE", "METRIC")
	lines := []string{shared.StyleHeader.Render(header)}

	if len(m.routes) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No routes found"))
		return strings.Join(lines, "\n")
	}

	visibleStart, visibleEnd := m.visibleRange(len(m.routes), maxLines-1)
	for i := visibleStart; i < visibleEnd; i++ {
		r := m.routes[i]
		row := fmt.Sprintf("  %-24s %-20s %-16s %-12s %-8d",
			truncate(r.NodeHostname, 24),
			truncate(r.Destination, 20),
			truncate(r.Gateway, 16),
			truncate(r.Interface, 12),
			r.Metric,
		)
		if i == m.cursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDNS(maxLines int) string {
	header := fmt.Sprintf("  %-24s %-32s",
		"NODE", "UPSTREAM ADDRESS")
	lines := []string{shared.StyleHeader.Render(header)}

	if len(m.dnsUpstreams) == 0 {
		lines = append(lines, shared.StyleMuted.Render("  No DNS upstreams found"))
		return strings.Join(lines, "\n")
	}

	visibleStart, visibleEnd := m.visibleRange(len(m.dnsUpstreams), maxLines-1)
	for i := visibleStart; i < visibleEnd; i++ {
		d := m.dnsUpstreams[i]
		row := fmt.Sprintf("  %-24s %-32s",
			truncate(d.NodeHostname, 24),
			truncate(d.Address, 32),
		)
		if i == m.cursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return strings.Join(lines, "\n")
}

// currentRowCount returns the number of rows in the currently active sub-tab.
func (m Model) currentRowCount() int {
	switch m.subTab {
	case subTabAddresses:
		return len(m.addresses)
	case subTabRoutes:
		return len(m.routes)
	case subTabDNS:
		return len(m.dnsUpstreams)
	}
	return 0
}

// visibleRange calculates the scrolled window of rows to display.
func (m Model) visibleRange(total, visible int) (start, end int) {
	if visible < 1 {
		visible = 1
	}
	// Keep cursor in view
	start = m.scrollY
	if m.cursor < start {
		start = m.cursor
	}
	if m.cursor >= start+visible {
		start = m.cursor - visible + 1
	}
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > total {
		end = total
	}
	return start, end
}

// --- Utility ---

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "\u2026"
}
