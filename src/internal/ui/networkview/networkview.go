// Package networkview provides the Network tab with Addresses, Routes, and DNS sub-tabs.
package networkview

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
)

type subTab int

const (
	subTabAddresses subTab = iota
	subTabRoutes
	subTabDNS
	numSubTabs
)

var subTabNames = [numSubTabs]string{"Addresses", "Routes", "DNS"}

type addressRow struct {
	Node     string
	LinkName string
	Address  string
	Family   string
	Scope    string
}

type routeRow struct {
	Node        string
	Destination string
	Gateway     string
	OutLinkName string
	Table       string
	Priority    uint32
}

type dnsRow struct {
	Node          string
	DNSServers    string
	SearchDomains string
}

// Internal messages.
type addressesLoadedMsg struct {
	rows []addressRow
	err  error
}

type routesLoadedMsg struct {
	rows []routeRow
	err  error
}

type dnsLoadedMsg struct {
	rows []dnsRow
	err  error
}

// Model is the network view model.
type Model struct {
	client    *talos.Client
	activeTab subTab

	addresses []addressRow
	routes    []routeRow
	dns       []dnsRow

	cursor       int
	scrollOff    int
	filter       string
	filterActive bool

	focusLeft bool // true = left pane focused, false = right pane

	loading         bool
	err             error
	width           int
	height          int
	refreshInterval time.Duration
}

// New creates a new network view model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		activeTab:       subTabAddresses,
		focusLeft:       true,
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
		return m.handleKey(msg)

	case shared.TickMsg:
		return m, m.fetchCurrentTab()

	case addressesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.addresses = msg.rows
			m.err = nil
		}
		m.loading = false

	case routesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.routes = msg.rows
			m.err = nil
		}
		m.loading = false

	case dnsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.dns = msg.rows
			m.err = nil
		}
		m.loading = false
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Tab):
		if m.focusLeft {
			m.activeTab = (m.activeTab + 1) % numSubTabs
			m.cursor = 0
			m.scrollOff = 0
			m.filter = ""
			m.loading = true
			return m, m.fetchCurrentTab()
		}
		// From right pane, Tab goes to left pane
		m.focusLeft = true

	case key.Matches(msg, shared.Keys.ShiftTab):
		if m.focusLeft {
			m.activeTab = (m.activeTab - 1 + numSubTabs) % numSubTabs
			m.cursor = 0
			m.scrollOff = 0
			m.filter = ""
			m.loading = true
			return m, m.fetchCurrentTab()
		}
		m.focusLeft = true

	case key.Matches(msg, shared.Keys.Back):
		if m.focusLeft {
			// Already on left pane, nothing to do
		} else {
			m.focusLeft = true
		}

	case key.Matches(msg, shared.Keys.Enter):
		if m.focusLeft {
			m.focusLeft = false
		}

	case key.Matches(msg, shared.Keys.Down):
		if !m.focusLeft {
			maxRow := m.rowCount() - 1
			if m.cursor < maxRow {
				m.cursor++
				m.adjustScroll()
			}
		}

	case key.Matches(msg, shared.Keys.Up):
		if !m.focusLeft {
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		}

	case key.Matches(msg, shared.Keys.PageDown):
		if !m.focusLeft {
			viewH := m.tableViewHeight()
			m.cursor += viewH
			maxRow := m.rowCount() - 1
			if maxRow < 0 {
				maxRow = 0
			}
			if m.cursor > maxRow {
				m.cursor = maxRow
			}
			m.adjustScroll()
		}

	case key.Matches(msg, shared.Keys.PageUp):
		if !m.focusLeft {
			viewH := m.tableViewHeight()
			m.cursor -= viewH
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.adjustScroll()
		}

	case key.Matches(msg, shared.Keys.Filter):
		if !m.focusLeft {
			m.filterActive = true
			m.filter = ""
		}
	}

	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Back):
		m.filterActive = false
		m.filter = ""
		m.cursor = 0
		m.scrollOff = 0

	case key.Matches(msg, shared.Keys.Enter):
		m.filterActive = false
		m.cursor = 0
		m.scrollOff = 0

	default:
		k := msg.String()
		if k == "backspace" {
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			}
		} else if len(k) == 1 {
			m.filter += k
		}
		m.cursor = 0
		m.scrollOff = 0
	}

	return m, nil
}

func (m Model) rowCount() int {
	switch m.activeTab {
	case subTabAddresses:
		return len(m.filteredAddresses())
	case subTabRoutes:
		return len(m.filteredRoutes())
	case subTabDNS:
		return len(m.filteredDNS())
	}
	return 0
}

func (m *Model) adjustScroll() {
	viewH := m.tableViewHeight()
	if m.cursor < m.scrollOff {
		m.scrollOff = m.cursor
	}
	if m.cursor >= m.scrollOff+viewH {
		m.scrollOff = m.cursor - viewH + 1
	}
}

func (m Model) tableViewHeight() int {
	// 1 line for header
	h := m.height - 1
	if h < 1 {
		return 1
	}
	return h
}

// View renders the network view.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	selectorWidth := m.width * 20 / 100
	if selectorWidth < 12 {
		selectorWidth = 12
	}
	tableWidth := m.width - selectorWidth - 1

	selector := m.renderSelector(selectorWidth)
	table := m.renderTable(tableWidth)

	selectorLines := strings.Split(selector, "\n")
	tableLines := strings.Split(table, "\n")

	maxH := m.height
	var combined []string
	for i := 0; i < maxH; i++ {
		left := ""
		if i < len(selectorLines) {
			left = selectorLines[i]
		}
		right := ""
		if i < len(tableLines) {
			right = tableLines[i]
		}
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

	for i := subTab(0); i < numSubTabs; i++ {
		prefix := "  "
		name := subTabNames[i]

		if i == m.activeTab {
			if !shared.PlainMode {
				prefix = " \u25b8"
			} else {
				prefix = " >"
			}
			if m.focusLeft {
				name = shared.StyleSelected.Render(name)
			} else {
				name = lipgloss.NewStyle().Foreground(shared.ColorPrimary).Render(name)
			}
		} else {
			name = shared.StyleMuted.Render(name)
		}

		lines = append(lines, prefix+" "+name)
	}

	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:m.height], "\n")
}

func (m Model) renderTable(width int) string {
	viewH := m.tableViewHeight()
	var lines []string

	if m.loading {
		lines = append(lines, shared.StyleMuted.Render(" Loading..."))
		for len(lines) < viewH+1 {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf(" Error: %v", m.err)))
		for len(lines) < viewH+1 {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	switch m.activeTab {
	case subTabAddresses:
		lines = m.renderAddressTable(width, viewH)
	case subTabRoutes:
		lines = m.renderRouteTable(width, viewH)
	case subTabDNS:
		lines = m.renderDNSTable(width, viewH)
	}

	// Filter status
	if m.filterActive {
		filterLine := fmt.Sprintf(" /:%s_", m.filter)
		if len(lines) > 0 {
			lines[len(lines)-1] = shared.StyleWarning.Render(filterLine)
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderAddressTable(width, viewH int) []string {
	rows := m.filteredAddresses()

	colNode := width * 25 / 100
	colIface := width * 15 / 100
	colAddr := width * 30 / 100
	colFamily := width * 15 / 100
	colScope := width - colNode - colIface - colAddr - colFamily

	header := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
		colNode, "NODE",
		colIface, "INTERFACE",
		colAddr, "ADDRESS",
		colFamily, "FAMILY",
		colScope, "SCOPE")
	lines := []string{shared.StyleHeader.Render(header)}

	for i := m.scrollOff; i < len(rows) && len(lines) <= viewH; i++ {
		r := rows[i]
		line := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
			colNode, truncate(r.Node, colNode),
			colIface, truncate(r.LinkName, colIface),
			colAddr, truncate(r.Address, colAddr),
			colFamily, truncate(r.Family, colFamily),
			colScope, truncate(r.Scope, colScope))
		if i == m.cursor && !m.focusLeft {
			line = shared.StyleSelected.Render(line)
		}
		lines = append(lines, line)
	}

	for len(lines) <= viewH {
		lines = append(lines, "")
	}

	return lines
}

func (m Model) renderRouteTable(width, viewH int) []string {
	rows := m.filteredRoutes()

	colNode := width * 20 / 100
	colDst := width * 22 / 100
	colGw := width * 22 / 100
	colIface := width * 14 / 100
	colTable := width * 10 / 100
	colMetric := width - colNode - colDst - colGw - colIface - colTable

	header := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s",
		colNode, "NODE",
		colDst, "DESTINATION",
		colGw, "GATEWAY",
		colIface, "INTERFACE",
		colTable, "TABLE",
		colMetric, "METRIC")
	lines := []string{shared.StyleHeader.Render(header)}

	for i := m.scrollOff; i < len(rows) && len(lines) <= viewH; i++ {
		r := rows[i]
		line := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*d",
			colNode, truncate(r.Node, colNode),
			colDst, truncate(r.Destination, colDst),
			colGw, truncate(r.Gateway, colGw),
			colIface, truncate(r.OutLinkName, colIface),
			colTable, truncate(r.Table, colTable),
			colMetric, r.Priority)
		if i == m.cursor && !m.focusLeft {
			line = shared.StyleSelected.Render(line)
		}
		lines = append(lines, line)
	}

	for len(lines) <= viewH {
		lines = append(lines, "")
	}

	return lines
}

func (m Model) renderDNSTable(width, viewH int) []string {
	rows := m.filteredDNS()

	colNode := width * 25 / 100
	colDNS := width * 40 / 100
	colSearch := width - colNode - colDNS

	header := fmt.Sprintf(" %-*s %-*s %-*s",
		colNode, "NODE",
		colDNS, "DNS SERVERS",
		colSearch, "SEARCH DOMAINS")
	lines := []string{shared.StyleHeader.Render(header)}

	for i := m.scrollOff; i < len(rows) && len(lines) <= viewH; i++ {
		r := rows[i]
		line := fmt.Sprintf(" %-*s %-*s %-*s",
			colNode, truncate(r.Node, colNode),
			colDNS, truncate(r.DNSServers, colDNS),
			colSearch, truncate(r.SearchDomains, colSearch))
		if i == m.cursor && !m.focusLeft {
			line = shared.StyleSelected.Render(line)
		}
		lines = append(lines, line)
	}

	for len(lines) <= viewH {
		lines = append(lines, "")
	}

	return lines
}

// Filter helpers.

func (m Model) filteredAddresses() []addressRow {
	if m.filter == "" {
		return m.addresses
	}
	f := strings.ToLower(m.filter)
	var out []addressRow
	for _, r := range m.addresses {
		if containsFilter(f, r.Node, r.LinkName, r.Address, r.Family, r.Scope) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) filteredRoutes() []routeRow {
	if m.filter == "" {
		return m.routes
	}
	f := strings.ToLower(m.filter)
	var out []routeRow
	for _, r := range m.routes {
		if containsFilter(f, r.Node, r.Destination, r.Gateway, r.OutLinkName, r.Table) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) filteredDNS() []dnsRow {
	if m.filter == "" {
		return m.dns
	}
	f := strings.ToLower(m.filter)
	var out []dnsRow
	for _, r := range m.dns {
		if containsFilter(f, r.Node, r.DNSServers, r.SearchDomains) {
			out = append(out, r)
		}
	}
	return out
}

func containsFilter(filter string, fields ...string) bool {
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), filter) {
			return true
		}
	}
	return false
}

// Data fetching.

func (m Model) fetchCurrentTab() tea.Cmd {
	switch m.activeTab {
	case subTabAddresses:
		return m.fetchAddresses()
	case subTabRoutes:
		return m.fetchRoutes()
	case subTabDNS:
		return m.fetchDNS()
	}
	return nil
}

func (m Model) fetchAddresses() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return addressesLoadedMsg{err: fmt.Errorf("no client connection")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nodes, err := cluster.GetMembers(ctx, client)
		if err != nil {
			return addressesLoadedMsg{err: err}
		}

		var rows []addressRow
		for _, node := range nodes {
			nodeRows, err := fetchNodeAddresses(ctx, client, node)
			if err != nil {
				shared.Debugf("[networkview] error fetching addresses for %s: %v", node.Hostname, err)
				continue
			}
			rows = append(rows, nodeRows...)
		}

		return addressesLoadedMsg{rows: rows}
	}
}

func fetchNodeAddresses(ctx context.Context, client *talos.Client, node cluster.NodeInfo) ([]addressRow, error) {
	nodeAddr := node.Hostname
	if len(node.Addresses) > 0 {
		nodeAddr = node.Addresses[0]
	}

	nodeCtx := talosclient.WithNodes(ctx, nodeAddr)

	meta := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.AddressStatusType,
		"",
		resource.VersionUndefined,
	)

	list, err := client.C.COSI.List(nodeCtx, meta)
	if err != nil {
		return nil, err
	}

	var rows []addressRow
	for _, item := range list.Items {
		addr, ok := item.(*networkres.AddressStatus)
		if !ok {
			continue
		}
		spec := addr.TypedSpec()
		rows = append(rows, addressRow{
			Node:     node.Hostname,
			LinkName: spec.LinkName,
			Address:  formatPrefix(spec.Address),
			Family:   spec.Family.String(),
			Scope:    spec.Scope.String(),
		})
	}

	return rows, nil
}

func (m Model) fetchRoutes() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return routesLoadedMsg{err: fmt.Errorf("no client connection")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nodes, err := cluster.GetMembers(ctx, client)
		if err != nil {
			return routesLoadedMsg{err: err}
		}

		var rows []routeRow
		for _, node := range nodes {
			nodeRows, err := fetchNodeRoutes(ctx, client, node)
			if err != nil {
				shared.Debugf("[networkview] error fetching routes for %s: %v", node.Hostname, err)
				continue
			}
			rows = append(rows, nodeRows...)
		}

		return routesLoadedMsg{rows: rows}
	}
}

func fetchNodeRoutes(ctx context.Context, client *talos.Client, node cluster.NodeInfo) ([]routeRow, error) {
	nodeAddr := node.Hostname
	if len(node.Addresses) > 0 {
		nodeAddr = node.Addresses[0]
	}

	nodeCtx := talosclient.WithNodes(ctx, nodeAddr)

	meta := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.RouteStatusType,
		"",
		resource.VersionUndefined,
	)

	list, err := client.C.COSI.List(nodeCtx, meta)
	if err != nil {
		return nil, err
	}

	var rows []routeRow
	for _, item := range list.Items {
		route, ok := item.(*networkres.RouteStatus)
		if !ok {
			continue
		}
		spec := route.TypedSpec()
		rows = append(rows, routeRow{
			Node:        node.Hostname,
			Destination: formatPrefix(spec.Destination),
			Gateway:     formatAddr(spec.Gateway),
			OutLinkName: spec.OutLinkName,
			Table:       spec.Table.String(),
			Priority:    spec.Priority,
		})
	}

	return rows, nil
}

func (m Model) fetchDNS() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return dnsLoadedMsg{err: fmt.Errorf("no client connection")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nodes, err := cluster.GetMembers(ctx, client)
		if err != nil {
			return dnsLoadedMsg{err: err}
		}

		var rows []dnsRow
		for _, node := range nodes {
			nodeRows, err := fetchNodeDNS(ctx, client, node)
			if err != nil {
				shared.Debugf("[networkview] error fetching DNS for %s: %v", node.Hostname, err)
				continue
			}
			rows = append(rows, nodeRows...)
		}

		return dnsLoadedMsg{rows: rows}
	}
}

func fetchNodeDNS(ctx context.Context, client *talos.Client, node cluster.NodeInfo) ([]dnsRow, error) {
	nodeAddr := node.Hostname
	if len(node.Addresses) > 0 {
		nodeAddr = node.Addresses[0]
	}

	nodeCtx := talosclient.WithNodes(ctx, nodeAddr)

	meta := resource.NewMetadata(
		networkres.NamespaceName,
		networkres.ResolverStatusType,
		"",
		resource.VersionUndefined,
	)

	list, err := client.C.COSI.List(nodeCtx, meta)
	if err != nil {
		return nil, err
	}

	var rows []dnsRow
	for _, item := range list.Items {
		resolver, ok := item.(*networkres.ResolverStatus)
		if !ok {
			continue
		}
		spec := resolver.TypedSpec()

		servers := make([]string, 0, len(spec.DNSServers))
		for _, s := range spec.DNSServers {
			servers = append(servers, s.String())
		}

		rows = append(rows, dnsRow{
			Node:          node.Hostname,
			DNSServers:    strings.Join(servers, ", "),
			SearchDomains: strings.Join(spec.SearchDomains, ", "),
		})
	}

	return rows, nil
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// ForceRefresh triggers an immediate data refresh for the active sub-tab.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchCurrentTab()
}

// Hints returns status bar hint text.
func (m Model) Hints() string {
	if m.focusLeft {
		return "tab/shift-tab:switch sub-tab  enter:table pane"
	}
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	return "esc:sub-tab pane  /:filter  up/down:navigate  pgup/pgdn:page"
}

// --- Utilities ---

func formatPrefix(p netip.Prefix) string {
	if !p.IsValid() {
		return ""
	}
	return p.String()
}

func formatAddr(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
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
