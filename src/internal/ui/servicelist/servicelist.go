// Package servicelist provides the cluster services tab view.
package servicelist

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

type sortField int

const (
	sortByNodeService sortField = iota
	sortByState
	sortByLastChange
	sortFieldMax
)

// ServiceListRow represents a single service on a single node.
type ServiceListRow struct {
	Node       string
	ServiceID  string
	State      string
	Health     string
	LastChange string
	LastEvent  string
}

// Internal messages.
type servicesLoadedMsg struct {
	rows []ServiceListRow
	err  error
}

// Model is the service list view model.
type Model struct {
	client          *talos.Client
	rows            []ServiceListRow
	filtered        []ServiceListRow
	cursor          int
	groupByNode     bool
	filter          string
	filterActive    bool
	detailView      bool
	detailEvents    []string
	loading         bool
	err             error
	width           int
	height          int
	scrollOff       int
	sortBy          sortField
	refreshInterval time.Duration
}

// New creates a new service list model.
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
		if m.filterActive {
			return m.updateFilter(msg)
		}
		if m.detailView {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)

	case shared.TickMsg:
		return m, m.fetchServices()

	case servicesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			prevCursor := m.cursor
			m.rows = msg.rows
			m.applyFilter()
			if prevCursor < len(m.filtered) {
				m.cursor = prevCursor
			}
		}
		m.loading = false
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
	case key.Matches(msg, shared.Keys.Enter):
		if m.cursor < len(m.filtered) {
			m.detailView = true
			m.detailEvents = nil
			row := m.filtered[m.cursor]
			if row.LastEvent != "" {
				m.detailEvents = []string{row.LastEvent}
			}
		}
	case key.Matches(msg, shared.Keys.GroupToggle):
		m.groupByNode = !m.groupByNode
	case key.Matches(msg, shared.Keys.Filter):
		m.filterActive = true
	case key.Matches(msg, shared.Keys.Sort):
		m.sortBy = (m.sortBy + 1) % sortFieldMax
		m.sortData()
	case key.Matches(msg, shared.Keys.ServiceRestart):
		return m, m.emitServiceRestart()
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
	if key.Matches(msg, shared.Keys.Back) {
		m.detailView = false
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = make([]ServiceListRow, len(m.rows))
		copy(m.filtered, m.rows)
	} else {
		lower := strings.ToLower(m.filter)
		m.filtered = nil
		for _, r := range m.rows {
			if strings.Contains(strings.ToLower(r.Node), lower) ||
				strings.Contains(strings.ToLower(r.ServiceID), lower) ||
				strings.Contains(strings.ToLower(r.State), lower) {
				m.filtered = append(m.filtered, r)
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
	case sortByNodeService:
		slices.SortFunc(m.filtered, func(a, b ServiceListRow) int {
			if c := cmp.Compare(a.Node, b.Node); c != 0 {
				return c
			}
			return cmp.Compare(a.ServiceID, b.ServiceID)
		})
	case sortByState:
		slices.SortFunc(m.filtered, func(a, b ServiceListRow) int {
			return cmp.Compare(a.State, b.State)
		})
	case sortByLastChange:
		slices.SortFunc(m.filtered, func(a, b ServiceListRow) int {
			return cmp.Compare(a.LastChange, b.LastChange)
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
	v := m.height - 3
	if v < 1 {
		return 1
	}
	return v
}

// View renders the service list.
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

	if m.loading && len(m.rows) == 0 {
		return shared.StyleMuted.Render("  Loading services...")
	}

	var lines []string

	// Filter row
	if m.filterActive {
		lines = append(lines, shared.StyleWarning.Render(fmt.Sprintf("  Filter: %s_", m.filter)))
	} else if m.filter != "" {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Filter: %s", m.filter)))
	}

	// Column header
	header := fmt.Sprintf("  %-20s %-16s %-12s %-8s %-40s",
		"NODE", "SERVICE", "STATE", "HEALTH", "LAST EVENT")
	lines = append(lines, shared.StyleHeader.Render(header))

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	visible := m.visibleRows()
	endIdx := m.scrollOff + visible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	lastNode := ""
	for i := m.scrollOff; i < endIdx; i++ {
		r := m.filtered[i]

		// Group header
		if m.groupByNode && r.Node != lastNode {
			lines = append(lines, shared.StyleHeader.Render(fmt.Sprintf("  --- %s ---", r.Node)))
			lastNode = r.Node
		}

		cursor := " "
		isCursor := false
		if i == m.cursor {
			cursor = ">"
			isCursor = true
		}

		stateIcon := shared.StatusIcon("Running")
		stateStyle := shared.StyleSuccess
		if r.State != "Running" {
			stateIcon = shared.StatusIcon("Stopped")
			stateStyle = shared.StyleWarning
		}
		if r.Health == "Failed" {
			stateIcon = shared.StatusIcon("Failed")
			stateStyle = shared.StyleError
		}

		healthStr := r.Health
		if healthStr == "" {
			healthStr = "?"
		}

		lastEvent := r.LastEvent
		if len(lastEvent) > 40 {
			lastEvent = lastEvent[:39] + "\u2026"
		}

		nodeDisplay := r.Node
		if m.groupByNode {
			nodeDisplay = "" // already shown in group header
		}

		row := fmt.Sprintf("%s %-20s %-16s %s %-10s %-8s %-40s",
			cursor,
			truncate(nodeDisplay, 20),
			r.ServiceID,
			stateStyle.Render(stateIcon),
			r.State,
			healthStr,
			lastEvent,
		)
		if isCursor {
			lines = append(lines, shared.StyleSelected.Render(row))
		} else {
			lines = append(lines, row)
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) viewDetail() string {
	r := m.filtered[m.cursor]
	var lines []string

	lines = append(lines, shared.StyleHeader.Render(fmt.Sprintf("  Service: %s on %s", r.ServiceID, r.Node)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-16s %s", shared.StyleLabel.Render("Service:"), r.ServiceID))
	lines = append(lines, fmt.Sprintf("  %-16s %s", shared.StyleLabel.Render("Node:"), r.Node))
	lines = append(lines, fmt.Sprintf("  %-16s %s", shared.StyleLabel.Render("State:"), r.State))
	lines = append(lines, fmt.Sprintf("  %-16s %s", shared.StyleLabel.Render("Health:"), r.Health))
	lines = append(lines, fmt.Sprintf("  %-16s %s", shared.StyleLabel.Render("Last Change:"), r.LastChange))

	lines = append(lines, "")
	lines = append(lines, shared.StyleLabel.Render("  Events:"))
	if len(m.detailEvents) == 0 {
		lines = append(lines, shared.StyleMuted.Render("    No events"))
	} else {
		for _, e := range m.detailEvents {
			lines = append(lines, fmt.Sprintf("    %s", e))
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
		return "esc:back"
	}
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	sortLabel := [sortFieldMax]string{"node/service", "state", "last change"}
	return fmt.Sprintf("enter:detail  /:filter  s:sort(%s)  g:group by node  ctrl+k:restart service", sortLabel[m.sortBy])
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchServices()
}

func (m Model) emitServiceRestart() tea.Cmd {
	if m.cursor >= len(m.filtered) {
		return nil
	}
	r := m.filtered[m.cursor]
	return func() tea.Msg {
		return shared.ServiceRestartRequestMsg{
			Node:      r.Node,
			ServiceID: r.ServiceID,
		}
	}
}

func (m Model) fetchServices() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if client == nil || client.C == nil {
			return servicesLoadedMsg{rows: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		targets, resolve := nodeTargets(ctx, client)
		nodeCtx := talosclient.WithNodes(ctx, targets...)
		resp, err := client.C.ServiceList(nodeCtx)
		if err != nil {
			return servicesLoadedMsg{err: err}
		}

		var rows []ServiceListRow
		for _, nodeMsg := range resp.GetMessages() {
			if nodeMsg.GetMetadata() == nil {
				continue
			}
			hostname := resolve(nodeMsg.GetMetadata().GetHostname())
			if hostname == "" {
				continue
			}
			for _, svc := range nodeMsg.GetServices() {
				health := "?"
				lastChange := ""
				lastEvent := ""

				if svc.GetHealth() != nil {
					if svc.GetHealth().GetUnknown() {
						health = "?"
					} else if svc.GetHealth().GetHealthy() {
						health = "OK"
					} else {
						health = "Failed"
					}
					if svc.GetHealth().GetLastChange() != nil {
						lastChange = svc.GetHealth().GetLastChange().AsTime().Format("15:04:05")
					}
					if svc.GetHealth().GetLastMessage() != "" {
						lastEvent = svc.GetHealth().GetLastMessage()
					}
				}

				if lastEvent == "" && svc.GetEvents() != nil {
					evts := svc.GetEvents().GetEvents()
					if len(evts) > 0 {
						last := evts[len(evts)-1]
						lastEvent = last.GetMsg()
					}
				}

				rows = append(rows, ServiceListRow{
					Node:       hostname,
					ServiceID:  svc.GetId(),
					State:      svc.GetState(),
					Health:     health,
					LastChange: lastChange,
					LastEvent:  lastEvent,
				})
			}
		}

		// Sort by node, then service ID
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Node != rows[j].Node {
				return rows[i].Node < rows[j].Node
			}
			return rows[i].ServiceID < rows[j].ServiceID
		})

		return servicesLoadedMsg{rows: rows}
	}
}

// BuildRows builds service list rows from raw data. Exported for testing.
func BuildRows(byNode map[string][]ServiceListRow) []ServiceListRow {
	var rows []ServiceListRow
	for _, svcs := range byNode {
		rows = append(rows, svcs...)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Node != rows[j].Node {
			return rows[i].Node < rows[j].Node
		}
		return rows[i].ServiceID < rows[j].ServiceID
	})
	return rows
}

func nodeTargets(ctx context.Context, client *talos.Client) (addrs []string, resolveHostname func(string) string) {
	identity := func(s string) string { return s }
	members, err := cluster.GetMembers(ctx, client)
	if err != nil || len(members) == 0 {
		return client.Endpoints, identity
	}
	addrToHost := make(map[string]string)
	for _, m := range members {
		for _, a := range m.Addresses {
			addrToHost[a] = m.Hostname
		}
		addrToHost[m.Hostname] = m.Hostname
		if len(m.Addresses) > 0 {
			addrs = append(addrs, m.Addresses[0])
		}
	}
	if len(addrs) == 0 {
		return client.Endpoints, identity
	}
	return addrs, func(s string) string {
		if h, ok := addrToHost[s]; ok {
			return h
		}
		return s
	}
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
