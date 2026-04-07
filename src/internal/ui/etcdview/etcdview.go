// Package etcdview provides the etcd members tab view.
package etcdview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

type etcdMemberRow struct {
	MemberID   uint64
	MemberHex  string
	Hostname   string
	PeerURLs   []string
	ClientURLs []string
	IsLearner  bool
}

type membersLoadedMsg struct {
	members []etcdMemberRow
	err     error
}

// Model is the etcd members view model.
type Model struct {
	client          *talos.Client
	members         []etcdMemberRow
	cursor          int
	scrollOff       int
	loading         bool
	err             error
	width           int
	height          int
	refreshInterval time.Duration
}

// New creates a new etcd view model.
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
		return m.updateList(msg)

	case shared.TickMsg:
		return m, m.fetchMembers()

	case membersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.members = msg.members
			m.err = nil
		}
		m.loading = false
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.Down):
		if m.cursor < len(m.members)-1 {
			m.cursor++
			m.adjustScroll()
		}
	case key.Matches(msg, shared.Keys.PageUp):
		m.cursor -= m.pageSize()
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.adjustScroll()
	case key.Matches(msg, shared.Keys.PageDown):
		m.cursor += m.pageSize()
		if m.cursor >= len(m.members) {
			m.cursor = len(m.members) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.adjustScroll()
	case key.Matches(msg, shared.Keys.EtcdRemoveMember):
		if m.cursor < len(m.members) {
			member := m.members[m.cursor]
			return m, func() tea.Msg {
				return shared.EtcdMemberRemoveRequestMsg{
					MemberID:  member.MemberID,
					MemberHex: member.MemberHex,
				}
			}
		}
	}
	return m, nil
}

func (m Model) pageSize() int {
	ps := m.height - 3 // header + borders
	if ps < 1 {
		return 1
	}
	return ps
}

func (m *Model) adjustScroll() {
	visible := m.pageSize()
	if m.cursor < m.scrollOff {
		m.scrollOff = m.cursor
	}
	if m.cursor >= m.scrollOff+visible {
		m.scrollOff = m.cursor - visible + 1
	}
}

func (m Model) fetchMembers() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := client.C.EtcdMemberList(ctx, &machine.EtcdMemberListRequest{
			QueryLocal: false,
		})
		if err != nil {
			return membersLoadedMsg{err: err}
		}

		seen := make(map[uint64]bool)
		var rows []etcdMemberRow
		for _, msg := range resp.GetMessages() {
			for _, m := range msg.GetMembers() {
				if seen[m.Id] {
					continue
				}
				seen[m.Id] = true
				rows = append(rows, etcdMemberRow{
					MemberID:   m.Id,
					MemberHex:  fmt.Sprintf("%x", m.Id),
					Hostname:   m.Hostname,
					PeerURLs:   m.PeerUrls,
					ClientURLs: m.ClientUrls,
					IsLearner:  m.IsLearner,
				})
			}
		}
		return membersLoadedMsg{members: rows}
	}
}

// View renders the etcd members list.
func (m Model) View() string {
	if m.loading && len(m.members) == 0 {
		return lipgloss.NewStyle().Padding(1, 2).Foreground(shared.ColorMuted).Render("Loading etcd members...")
	}
	if m.err != nil && len(m.members) == 0 {
		return lipgloss.NewStyle().Padding(1, 2).Foreground(shared.ColorError).Render(
			fmt.Sprintf("Error: %v", m.err))
	}
	if len(m.members) == 0 {
		return lipgloss.NewStyle().Padding(1, 2).Foreground(shared.ColorMuted).Render("No etcd members found")
	}

	var b strings.Builder

	// Header
	header := fmt.Sprintf("  %-18s %-20s %-30s %-30s %s",
		"MEMBER ID", "HOSTNAME", "PEER URLS", "CLIENT URLS", "LEARNER")
	b.WriteString(shared.StyleHeader.Render(header))
	b.WriteString("\n")

	// Rows
	visible := m.pageSize()
	end := m.scrollOff + visible
	if end > len(m.members) {
		end = len(m.members)
	}

	for i := m.scrollOff; i < end; i++ {
		row := m.members[i]
		peerURLs := strings.Join(row.PeerURLs, ",")
		clientURLs := strings.Join(row.ClientURLs, ",")
		if len(peerURLs) > 28 {
			peerURLs = peerURLs[:25] + "..."
		}
		if len(clientURLs) > 28 {
			clientURLs = clientURLs[:25] + "..."
		}

		learner := ""
		if row.IsLearner {
			learner = "yes"
		}

		line := fmt.Sprintf("  %-18s %-20s %-30s %-30s %s",
			row.MemberHex, row.Hostname, peerURLs, clientURLs, learner)

		if i == m.cursor {
			b.WriteString(shared.StyleSelected.Width(m.width).Render(line))
		} else {
			b.WriteString(line)
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// ForceRefresh triggers an immediate data fetch.
func (m Model) ForceRefresh() tea.Cmd {
	return m.fetchMembers()
}

// Hints returns the status bar hint text for this view.
func (m Model) Hints() string {
	return "↑↓:navigate  ctrl+m:remove member  ctrl+r:refresh"
}
