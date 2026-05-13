// Package etcdview provides the etcd tab UI for lazytalos.
package etcdview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/etcd"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// Sub-tab indices.
const (
	subTabMembers = 0
	subTabConfig  = 1
	subTabCount   = 2
)

var subTabNames = []string{"Members", "Config"}

// membersLoadedMsg is the internal message returned when member data is fetched.
type membersLoadedMsg struct {
	members []etcd.Member
	err     error
}

// Model is the etcd tab view model.
type Model struct {
	client          *talos.Client
	subTab          int
	members         []etcd.Member
	loading         bool
	err             error
	cursor          int
	width, height   int
	refreshInterval time.Duration
}

// New creates a new etcdview model.
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
		return m.updateKey(msg)

	case shared.TickMsg:
		return m, m.ForceRefresh()

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

func (m Model) updateKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.SubTabPrev):
		m.subTab = (m.subTab - 1 + subTabCount) % subTabCount
		m.cursor = 0
	case key.Matches(msg, shared.Keys.SubTabNext):
		m.subTab = (m.subTab + 1) % subTabCount
		m.cursor = 0
	case key.Matches(msg, shared.Keys.Up):
		if m.subTab == subTabMembers && m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, shared.Keys.Down):
		if m.subTab == subTabMembers && m.cursor < len(m.members)-1 {
			m.cursor++
		}
	case key.Matches(msg, shared.Keys.RemoveEtcdMember):
		if m.subTab == subTabMembers && m.cursor >= 0 && m.cursor < len(m.members) {
			member := m.members[m.cursor]
			return m, func() tea.Msg {
				return shared.EtcdMemberRemoveRequestMsg{
					Node:     member.NodeHostname,
					MemberID: member.MemberID,
				}
			}
		}
		// Left/Right: do not consume — let parent handle tab switching.
	}
	return m, nil
}

// View renders the etcd tab.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	// Sub-tab bar.
	sb.WriteString(renderSubTabs(m.subTab))
	sb.WriteString("\n\n")

	switch m.subTab {
	case subTabMembers:
		sb.WriteString(m.viewMembers())
	case subTabConfig:
		sb.WriteString(m.viewConfig())
	}

	return sb.String()
}

func renderSubTabs(active int) string {
	var parts []string
	for i, name := range subTabNames {
		if i == active {
			parts = append(parts, shared.StyleTabActive.Render(name))
		} else {
			parts = append(parts, shared.StyleTabInactive.Render(name))
		}
	}
	return "  " + strings.Join(parts, "  ")
}

func (m Model) viewMembers() string {
	if m.loading && len(m.members) == 0 {
		return shared.StyleMuted.Render("  Loading etcd members...")
	}

	var lines []string

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	// Column widths.
	idW := 18
	hostW := 22
	peerW := 24
	clientW := 24

	header := fmt.Sprintf("  %-*s %-*s %-*s %-*s %s",
		idW, "MEMBER ID",
		hostW, "HOSTNAME",
		peerW, "PEER ADDRESS",
		clientW, "CLIENT ADDRESS",
		"LEADER",
	)
	lines = append(lines, shared.StyleHeader.Render(header))

	if len(m.members) == 0 && !m.loading {
		lines = append(lines, shared.StyleMuted.Render("  No etcd members found."))
		return strings.Join(lines, "\n")
	}

	leaderIcon := "★"
	if shared.IsPlainMode() {
		leaderIcon = "*"
	}

	for i, mem := range m.members {
		peerAddr := ""
		if len(mem.PeerAddrs) > 0 {
			peerAddr = mem.PeerAddrs[0]
		}
		clientAddr := ""
		if len(mem.ClientAddrs) > 0 {
			clientAddr = mem.ClientAddrs[0]
		}
		leader := ""
		if mem.IsLeader {
			leader = leaderIcon
		}

		memberIDHex := fmt.Sprintf("%x", mem.MemberID)

		row := fmt.Sprintf("  %-*s %-*s %-*s %-*s %s",
			idW, shared.Truncate(memberIDHex, idW),
			hostW, shared.Truncate(shared.ShortenHostname(mem.Hostname), hostW),
			peerW, shared.Truncate(peerAddr, peerW),
			clientW, shared.Truncate(clientAddr, clientW),
			leader,
		)

		isCursor := i == m.cursor
		if mem.IsLeader {
			row = shared.StyleSuccess.Render(row)
		} else if isCursor {
			row = shared.StyleSelected.Render(row)
		}
		if isCursor && !mem.IsLeader {
			// Already styled by StyleSelected above; add cursor marker.
		} else if isCursor && mem.IsLeader {
			// Leader + cursor: wrap in selected to indicate focus.
			row = shared.StyleSelected.Render(row)
		}
		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func (m Model) viewConfig() string {
	if m.loading && len(m.members) == 0 {
		return shared.StyleMuted.Render("  Loading etcd members...")
	}

	var lines []string

	if m.err != nil {
		lines = append(lines, shared.StyleError.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	summary := fmt.Sprintf("etcd cluster: %d member(s)", len(m.members))
	lines = append(lines, shared.StyleHeader.Render("  "+summary))
	lines = append(lines, "")

	for _, mem := range m.members {
		idHex := fmt.Sprintf("%x", mem.MemberID)
		lines = append(lines, shared.StyleValue.Render(fmt.Sprintf("  Member %s", idHex)))

		peerURLs := strings.Join(mem.PeerAddrs, ", ")
		clientURLs := strings.Join(mem.ClientAddrs, ", ")
		if peerURLs == "" {
			peerURLs = "(none)"
		}
		if clientURLs == "" {
			clientURLs = "(none)"
		}

		lines = append(lines, fmt.Sprintf("    %s %s",
			shared.StyleLabel.Render("Hostname:"),
			shared.StyleValue.Render(mem.Hostname),
		))
		lines = append(lines, fmt.Sprintf("    %s %s",
			shared.StyleLabel.Render("Peer URLs:"),
			shared.StyleValue.Render(peerURLs),
		))
		lines = append(lines, fmt.Sprintf("    %s %s",
			shared.StyleLabel.Render("Client URLs:"),
			shared.StyleValue.Render(clientURLs),
		))
		lines = append(lines, fmt.Sprintf("    %s %s",
			shared.StyleLabel.Render("Learner:"),
			shared.StyleValue.Render(fmt.Sprintf("%v", mem.IsLearner)),
		))
		lines = append(lines, "")
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
	switch m.subTab {
	case subTabMembers:
		return "</> sub-tabs  ↑↓:select  ctrl+m:remove member"
	case subTabConfig:
		return "</> sub-tabs"
	}
	return ""
}

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		members, err := etcd.ListMembers(ctx, client)
		return membersLoadedMsg{members: members, err: err}
	}
}
