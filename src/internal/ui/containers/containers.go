// Package containers provides the Containers tab UI for lazytalos.
package containers

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/node"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

type sortField int

const (
	sortByNodeName sortField = iota
	sortByState
	sortByImage
	sortFieldMax
)

// Internal messages.
type containersLoadedMsg struct {
	containers []node.Container
	err        error
}

// Model is the containers list view model.
type Model struct {
	client          *talos.Client
	containers      []node.Container // all fetched
	filtered        []node.Container // after filter
	cursor          int
	scrollOff       int
	filter          string
	filterActive    bool
	detailView      bool
	loading         bool
	err             error
	width, height   int
	sortBy          sortField
	refreshInterval time.Duration

	// nodeFilter cycles through node names; empty means "all"
	nodeFilter string
}

// New creates a new containers model.
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
		return m, m.ForceRefresh()

	case containersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			prevCursor := m.cursor
			m.containers = msg.containers
			m.applyFilter()
			// Preserve cursor position across refreshes
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
	case key.Matches(msg, shared.Keys.Enter):
		if m.cursor < len(m.filtered) {
			m.detailView = true
		}
	case key.Matches(msg, shared.Keys.Back):
		if m.filter != "" {
			m.filter = ""
			m.applyFilter()
		}
	case key.Matches(msg, shared.Keys.Filter):
		m.filterActive = true
	case key.Matches(msg, shared.Keys.ContainerLogs):
		if m.cursor < len(m.filtered) {
			c := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return shared.ContainerLogsRequestMsg{
					Node:        c.NodeHostname,
					Namespace:   c.Namespace,
					ContainerID: c.Name,
				}
			}
		}
	case key.Matches(msg, shared.Keys.Sort):
		m.sortBy = (m.sortBy + 1) % sortFieldMax
		m.sortData()
	default:
		if msg.String() == "n" {
			m.cycleNodeFilter()
		}
	}
	return m, nil
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
	v := m.height - 3 // header + column header + possible filter
	if v < 1 {
		return 1
	}
	return v
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

// cycleNodeFilter cycles through unique node names (or clears the filter).
func (m *Model) cycleNodeFilter() {
	nodes := m.uniqueNodes()
	if len(nodes) == 0 {
		return
	}
	if m.nodeFilter == "" {
		m.nodeFilter = nodes[0]
	} else {
		found := false
		for i, n := range nodes {
			if n == m.nodeFilter {
				if i+1 < len(nodes) {
					m.nodeFilter = nodes[i+1]
				} else {
					m.nodeFilter = ""
				}
				found = true
				break
			}
		}
		if !found {
			m.nodeFilter = ""
		}
	}
	m.applyFilter()
}

// uniqueNodes returns sorted unique node hostnames from all containers.
func (m *Model) uniqueNodes() []string {
	seen := make(map[string]bool)
	var result []string
	for _, c := range m.containers {
		if !seen[c.NodeHostname] {
			seen[c.NodeHostname] = true
			result = append(result, c.NodeHostname)
		}
	}
	return result
}

func (m *Model) sortData() {
	switch m.sortBy {
	case sortByNodeName:
		slices.SortFunc(m.filtered, func(a, b node.Container) int {
			if c := cmp.Compare(a.NodeHostname, b.NodeHostname); c != 0 {
				return c
			}
			return cmp.Compare(a.Name, b.Name)
		})
	case sortByState:
		slices.SortFunc(m.filtered, func(a, b node.Container) int {
			return cmp.Compare(a.State, b.State)
		})
	case sortByImage:
		slices.SortFunc(m.filtered, func(a, b node.Container) int {
			return cmp.Compare(a.Image, b.Image)
		})
	}
}

func (m *Model) applyFilter() {
	lower := strings.ToLower(m.filter)
	m.filtered = nil
	for _, c := range m.containers {
		if m.nodeFilter != "" && c.NodeHostname != m.nodeFilter {
			continue
		}
		if lower != "" && !strings.Contains(strings.ToLower(c.Name), lower) {
			continue
		}
		m.filtered = append(m.filtered, c)
	}
	m.sortData()
	m.cursor = 0
}

// View renders the containers tab.
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

	if m.loading && len(m.containers) == 0 {
		return shared.StyleMuted.Render("  Loading containers...")
	}

	var lines []string

	// Filter / node-filter row
	if m.filterActive {
		lines = append(lines, shared.StyleWarning.Render(fmt.Sprintf("  Filter: %s_", m.filter)))
	} else if m.filter != "" {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Filter: %s", m.filter)))
	} else if m.nodeFilter != "" {
		lines = append(lines, shared.StyleMuted.Render(fmt.Sprintf("  Node: %s", m.nodeFilter)))
	}

	// Column widths
	nodeW, nsW, nameW, imageW, stateW, pidW := columnWidths(m.width)

	// Header
	header := fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s %-*s",
		nodeW, "NODE",
		nsW, "NAMESPACE",
		nameW, "NAME",
		imageW, "IMAGE",
		stateW, "STATE",
		pidW, "PID",
	)
	lines = append(lines, shared.StyleHeader.Render(header))

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	if len(m.filtered) == 0 && !m.loading {
		lines = append(lines, shared.StyleMuted.Render("  No containers found."))
		return strings.Join(lines, "\n")
	}

	visible := m.height - len(lines) - 1
	if visible < 1 {
		visible = 1
	}
	endIdx := m.scrollOff + visible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	for i := m.scrollOff; i < endIdx; i++ {
		c := m.filtered[i]
		isCursor := i == m.cursor

		cursor := " "
		if isCursor {
			cursor = ">"
		}

		stateStr := shared.Truncate(c.State, stateW)
		var stateRendered string
		if strings.ToUpper(c.State) == "RUNNING" {
			stateRendered = shared.StyleSuccess.Render(stateStr)
		} else {
			stateRendered = shared.StyleWarning.Render(stateStr)
		}

		pidStr := ""
		if c.PID > 0 {
			pidStr = fmt.Sprintf("%d", c.PID)
		}

		row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s",
			cursor,
			nodeW, shared.Truncate(shared.ShortenHostname(c.NodeHostname), nodeW),
			nsW, shared.Truncate(c.Namespace, nsW),
			nameW, shared.Truncate(c.Name, nameW),
			imageW, shared.Truncate(c.Image, imageW),
			stateW, stateRendered,
			pidW, shared.Truncate(pidStr, pidW),
		)
		lines = append(lines, renderRow(isCursor, row))
	}

	return strings.Join(lines, "\n")
}

func (m Model) viewDetail() string {
	c := m.filtered[m.cursor]
	var lines []string

	lines = append(lines, shared.StyleHeader.Render(fmt.Sprintf("  Container: %s", c.Name)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("Node:"), shared.StyleValue.Render(shared.ShortenHostname(c.NodeHostname))))
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("Namespace:"), shared.StyleValue.Render(c.Namespace)))
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("Name:"), shared.StyleValue.Render(c.Name)))
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("Image:"), shared.StyleValue.Render(c.FullImage)))
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("State:"), renderState(c.State)))
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("Status:"), shared.StyleValue.Render(c.Status)))
	lines = append(lines, fmt.Sprintf("  %s %s", shared.StyleLabel.Render("PID:"), shared.StyleValue.Render(fmt.Sprintf("%d", c.PID))))
	lines = append(lines, "")
	lines = append(lines, shared.StyleMuted.Render("  Press Esc to go back  •  ctrl+l: view logs"))

	return strings.Join(lines, "\n")
}

func renderState(state string) string {
	if strings.ToUpper(state) == "RUNNING" {
		return shared.StyleSuccess.Render(state)
	}
	return shared.StyleWarning.Render(state)
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Hints returns status bar hint text.
func (m Model) Hints() string {
	if m.detailView {
		return "esc:back  ctrl+l:logs"
	}
	if m.filterActive {
		return "type to filter  enter:apply  esc:cancel"
	}
	sortLabel := [sortFieldMax]string{"node/name", "state", "image"}
	return fmt.Sprintf("enter:detail  /:filter  s:sort(%s)  n:node-filter  ctrl+l:logs", sortLabel[m.sortBy])
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		containers, err := node.ListContainers(ctx, client)
		return containersLoadedMsg{containers: containers, err: err}
	}
}

// columnWidths returns column widths based on total available width.
// Minimum useful widths: Node=20, Namespace=8, Name=28, Image=22, State=10, PID=6
func columnWidths(total int) (nodeW, nsW, nameW, imageW, stateW, pidW int) {
	nodeW = 20
	nsW = 8
	nameW = 28
	imageW = 22
	stateW = 10
	pidW = 6

	// 2 (indent) + 1 (cursor) + spaces between = total overhead of ~10
	// Try to expand name/image if space allows
	used := 2 + 1 + nodeW + 1 + nsW + 1 + nameW + 1 + imageW + 1 + stateW + 1 + pidW
	extra := total - used
	if extra > 0 {
		// Give half extra to name, quarter to image
		nameW += extra / 2
		imageW += extra / 4
	}
	return
}

func renderRow(highlighted bool, s string) string {
	if highlighted {
		return shared.StyleSelected.Render(s)
	}
	return s
}

