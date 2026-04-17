// Package clustergrid renders a full-screen overlay that shows a compact
// health card for every context defined in ~/.talos/config.
package clustergrid

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

type cardStatus int

const (
	cardLoading cardStatus = iota
	cardReady
	cardError
)

// card is the per-context state tracked by the grid.
type card struct {
	context string
	status  cardStatus
	summary *clusterSummary
	err     error
	client  *talos.Client // short-lived, owned by this card; nil until fetch returns
	reused  bool          // borrowed from parent app — never close on grid dismiss
}

// Model is the Bubble Tea model for the multi-cluster grid overlay.
type Model struct {
	cards        []card
	cursor       int
	width        int
	height       int
	talosconfig  string
	activeCtx    string        // parent app's currently-connected context (for reuse)
	activeClient *talos.Client // borrowed; never closed here
	closed       bool          // set by Close() so stale messages drop cleanly

	// currentGen is the most recent fetch-dispatch generation. Messages
	// whose gen doesn't match are stale (from a preempted refresh) and
	// their clients are released without applying them.
	currentGen int
}

// New constructs a grid model. `activeClient` (if non-nil) is the currently
// connected Talos client in the parent app; its context is reused rather
// than re-dialed. `activeCtx` is its context name.
func New(talosconfig string, activeClient *talos.Client, activeCtx string) Model {
	names, _, _ := talos.ListContextNames(talosconfig)

	m := Model{
		talosconfig:  talosconfig,
		activeCtx:    activeCtx,
		activeClient: activeClient,
		cards:        make([]card, 0, len(names)),
		currentGen:   1,
	}
	for _, name := range names {
		m.cards = append(m.cards, card{context: name, status: cardLoading})
	}
	for i, c := range m.cards {
		if c.context == activeCtx {
			m.cursor = i
			break
		}
	}
	return m
}

// SetSize updates the grid's rendering dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Init fires off the initial per-cluster fetch fan-out.
func (m Model) Init() tea.Cmd {
	return m.fanOutFetch()
}

// Close releases any non-reused short-lived clients owned by the grid.
// Safe to call multiple times.
func (m *Model) Close() {
	m.closed = true
	for i := range m.cards {
		c := &m.cards[i]
		if c.client != nil && !c.reused {
			_ = c.client.Close()
			c.client = nil
		}
	}
}

// Update handles messages routed to the grid overlay.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.closed {
			return m, nil
		}
		switch {
		case key.Matches(msg, shared.Keys.Back) ||
			key.Matches(msg, shared.Keys.ClusterGrid):
			return m, func() tea.Msg { return ClosedMsg{} }
		case key.Matches(msg, shared.Keys.Enter):
			if len(m.cards) == 0 {
				return m, nil
			}
			sel := m.cards[m.cursor].context
			return m, func() tea.Msg { return ClosedMsg{Selected: sel} }
		case key.Matches(msg, shared.Keys.Tab):
			m.cursor = nextCursor(m.cursor, len(m.cards))
		case key.Matches(msg, shared.Keys.ShiftTab):
			m.cursor = prevCursor(m.cursor, len(m.cards))
		case msg.String() == "r":
			m.currentGen++
			return m, m.fanOutFetch()
		}

	case CardReadyMsg:
		// Always handle so both closed-and-dismissed and stale-refresh
		// messages release any non-reused client they carry.
		if m.closed || msg.gen != m.currentGen {
			if msg.client != nil && !msg.reused {
				_ = msg.client.Close()
			}
			return m, nil
		}
		if idx := m.findCard(msg.contextName); idx >= 0 {
			old := m.cards[idx].client
			if old != nil && !m.cards[idx].reused && old != msg.client {
				_ = old.Close()
			}
			s := msg.summary
			m.cards[idx].status = cardReady
			m.cards[idx].summary = &s
			m.cards[idx].err = nil
			m.cards[idx].client = msg.client
			m.cards[idx].reused = msg.reused
		}

	case CardErrMsg:
		if m.closed || msg.gen != m.currentGen {
			if msg.client != nil && !msg.reused {
				_ = msg.client.Close()
			}
			return m, nil
		}
		if idx := m.findCard(msg.contextName); idx >= 0 {
			old := m.cards[idx].client
			if old != nil && !m.cards[idx].reused && old != msg.client {
				_ = old.Close()
			}
			m.cards[idx].status = cardError
			m.cards[idx].err = msg.err
			m.cards[idx].client = msg.client
			m.cards[idx].reused = msg.reused
		}

	case refreshAllMsg:
		m.currentGen++
		return m, m.fanOutFetch()
	}

	return m, nil
}

// Hints returns the status bar hint text for the grid overlay.
func (m Model) Hints() string {
	return "tab/shift+tab:nav cards  enter:Dashboard  2-9:tab  r:refresh  esc/1:close  q:quit"
}

// FocusedContext returns the context name of the card currently under the
// cursor, or "" when there are no cards. Exposed so the app can drill into
// the focused cluster in response to keys the grid itself does not handle
// (e.g. digit keys 1-8 that target a specific tab).
func (m Model) FocusedContext() string {
	if len(m.cards) == 0 || m.cursor < 0 || m.cursor >= len(m.cards) {
		return ""
	}
	return m.cards[m.cursor].context
}

// fanOutFetch dispatches one fetch command per card, tagged with the grid's
// current generation so late arrivals from a preempted refresh are dropped.
func (m Model) fanOutFetch() tea.Cmd {
	if len(m.cards) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(m.cards))
	for _, c := range m.cards {
		cmds = append(cmds, m.fetchCluster(c.context, m.currentGen))
	}
	return tea.Batch(cmds...)
}

func (m Model) findCard(contextName string) int {
	for i := range m.cards {
		if m.cards[i].context == contextName {
			return i
		}
	}
	return -1
}

func prevCursor(cur, n int) int {
	if n == 0 {
		return 0
	}
	if cur == 0 {
		return n - 1
	}
	return cur - 1
}

func nextCursor(cur, n int) int {
	if n == 0 {
		return 0
	}
	if cur >= n-1 {
		return 0
	}
	return cur + 1
}
