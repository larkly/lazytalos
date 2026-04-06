package app

import (
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/larkly/lazytalos/internal/ui/contextpicker"
	"github.com/larkly/lazytalos/internal/ui/dashboard"
	"github.com/larkly/lazytalos/internal/ui/logviewer"
	"github.com/larkly/lazytalos/internal/ui/modal"
	"github.com/larkly/lazytalos/internal/ui/nodelist"
	"github.com/larkly/lazytalos/internal/ui/servicelist"
	"github.com/larkly/lazytalos/internal/ui/statusbar"
)

type activeView int

const (
	viewContextPicker activeView = iota
	viewDashboard
	viewNodeList
	viewServiceList
	viewLogViewer
)

type modalType int

const (
	modalNone modalType = iota
	modalConfirm
	modalError
)

// Model is the root application model.
type Model struct {
	// Connection state
	client      *talos.Client
	contextName string

	// Context picker (pre-connection)
	contextPicker contextpicker.Model

	// Tab system
	tabs      []TabDef
	activeTab int
	tabInited []bool

	// Views
	dashboard   dashboard.Model
	nodeList    nodelist.Model
	serviceList servicelist.Model
	logViewer   logviewer.Model

	// Selection (for bulk node ops)
	selectedNodes map[string]bool

	// Modals
	activeModal modalType
	confirm     modal.ConfirmModel
	errModal    modal.ErrorModel

	// UI
	statusBar statusbar.Model
	view      activeView

	// Sizing
	width  int
	height int

	// Config
	refreshInterval time.Duration
	talosconfig     string
	pickContext     bool // always show picker

	// State
	restart    bool
	autoSelect string // context to auto-select on Init
	version    string
}

// Options configures the application.
type Options struct {
	Talosconfig     string
	Context         string
	RefreshInterval time.Duration
	PickContext     bool
	Version        string
	Plain          bool
}

// ShouldRestart returns true if the app quit due to a restart request.
func (m Model) ShouldRestart() bool {
	return m.restart
}

// New creates the root model.
func New(opts Options) Model {
	shared.PlainMode = opts.Plain

	refresh := opts.RefreshInterval
	if refresh == 0 {
		refresh = 5 * time.Second
	}

	tabs := defaultTabs()

	contexts, _, err := talos.ListContextNames(opts.Talosconfig)
	if err != nil {
		shared.Debugf("[app] error listing contexts: %v", err)
	} else {
		shared.Debugf("[app] found %d contexts", len(contexts))
	}

	m := Model{
		view:            viewContextPicker,
		statusBar:       statusbar.Model{Width: 0},
		tabs:            tabs,
		tabInited:       make([]bool, len(tabs)),
		refreshInterval: refresh,
		talosconfig:     opts.Talosconfig,
		pickContext:     opts.PickContext,
		version:        opts.Version,
		selectedNodes:  make(map[string]bool),
	}

	// Auto-select if --context flag is set, or exactly one context and not forced to pick
	if opts.Context != "" && !opts.PickContext {
		m.autoSelect = opts.Context
		m.contextPicker = contextpicker.New(contexts, err)
	} else if err == nil && len(contexts) == 1 && !opts.PickContext {
		m.autoSelect = contexts[0]
		m.contextPicker = contextpicker.New(contexts, nil)
	} else {
		m.contextPicker = contextpicker.New(contexts, err)
	}

	m.statusBar.CurrentView = "contextpicker"
	m.statusBar.Hint = "Select a context to connect"

	return m
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	if m.autoSelect != "" {
		name := m.autoSelect
		return func() tea.Msg {
			return shared.ContextSelectedMsg{ContextName: name}
		}
	}
	return nil
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.contextPicker.SetSize(m.width, m.height)
		m.confirm.SetSize(m.width, m.height)
		m.errModal.SetSize(m.width, m.height)
		m.statusBar.Width = m.width
		return m.updateActiveView(msg)

	case tea.KeyMsg:
		// Modal intercepts all keys
		if m.activeModal != modalNone {
			return m.updateModal(msg)
		}

		switch {
		case key.Matches(msg, shared.Keys.Quit) && m.view != viewContextPicker:
			return m, tea.Quit
		case key.Matches(msg, shared.Keys.ContextPicker) && m.view != viewContextPicker:
			return m.switchToContextPicker()
		}

		// Tab switching (only from top-level views)
		if m.isTopLevelView() {
			if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '4' {
				idx := int(s[0] - '1')
				if idx < len(m.tabs) {
					return m.switchTab(idx)
				}
			}
			switch {
			case key.Matches(msg, shared.Keys.Right):
				next := (m.activeTab + 1) % len(m.tabs)
				return m.switchTab(next)
			case key.Matches(msg, shared.Keys.Left):
				prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				return m.switchTab(prev)
			}
		}

		// Global force refresh
		if key.Matches(msg, shared.Keys.Refresh) && m.view != viewContextPicker {
			return m.forceRefreshActiveView()
		}

		return m.updateActiveView(msg)

	case shared.ContextSelectedMsg:
		shared.Debugf("[app] context selected: %s", msg.ContextName)
		m.contextName = msg.ContextName
		m.statusBar.Context = msg.ContextName
		m.statusBar.Hint = "Connecting..."
		return m, m.connectToContext(msg.ContextName)

	case shared.ClientConnectedMsg:
		shared.Debugf("[app] connected to context=%s", msg.ContextName)
		m.client = msg.Client
		m.contextName = msg.ContextName
		m.statusBar.Context = msg.ContextName
		m.statusBar.Connected = true
		// Switch to dashboard tab (first tab)
		m, cmd := m.switchTab(0)
		return m, tea.Batch(cmd, m.refreshTickCmd())

	case shared.ClientConnectErrMsg:
		shared.Debugf("[app] connect error: %v", msg.Err)
		m.errModal = modal.NewError("Connection failed", msg.Err)
		m.errModal.SetSize(m.width, m.height)
		m.activeModal = modalError
		return m, nil

	case modal.ErrorDismissedMsg:
		m.activeModal = modalNone
		// Return to context picker after connection error
		if m.client == nil {
			return m.switchToContextPicker()
		}
		return m, nil

	case modal.ConfirmAction:
		m.activeModal = modalNone
		if !msg.Confirm {
			return m, nil
		}
		return m.handleConfirmedAction(msg)

	case shared.NodeActionMsg:
		shared.Debugf("[app] action completed: %s on %v", msg.Action, msg.Nodes)
		m.statusBar.Hint = msg.Action + " completed"
		return m.forceRefreshActiveView()

	case shared.NodeActionErrMsg:
		shared.Debugf("[app] action error: %s: %v", msg.Action, msg.Err)
		m.errModal = modal.NewError(msg.Action+" failed", msg.Err)
		m.errModal.SetSize(m.width, m.height)
		m.activeModal = modalError
		return m, nil

	case shared.TickMsg:
		m2, viewCmd := m.updateAllViews(msg)
		return m2, tea.Batch(viewCmd, m.refreshTickCmd())
	}

	return m.updateActiveView(msg)
}

func (m Model) handleConfirmedAction(action modal.ConfirmAction) (Model, tea.Cmd) {
	nodes := make([]string, len(action.Nodes))
	for i, n := range action.Nodes {
		nodes[i] = n.ID
	}
	if len(nodes) == 0 && action.Node != "" {
		nodes = []string{action.Node}
	}

	switch action.Action {
	case "reboot":
		return m, m.rebootNodes(nodes)
	case "shutdown":
		return m, m.shutdownNodes(nodes)
	}
	return m, nil
}

func (m Model) forceRefreshActiveView() (Model, tea.Cmd) {
	shared.Debugf("[app] forceRefreshActiveView: view=%d", m.view)
	switch m.view {
	case viewDashboard:
		return m, m.dashboard.ForceRefresh()
	case viewNodeList:
		return m, m.nodeList.ForceRefresh()
	case viewServiceList:
		return m, m.serviceList.ForceRefresh()
	case viewLogViewer:
		return m, m.logViewer.ForceRefresh()
	}
	return m, nil
}
