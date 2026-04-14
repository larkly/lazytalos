package app

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/config"
	"github.com/larkly/lazytalos/internal/etcd"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/larkly/lazytalos/internal/update"
	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/ui/configeditor"
	"github.com/larkly/lazytalos/internal/ui/containers"
	"github.com/larkly/lazytalos/internal/ui/contextpicker"
	"github.com/larkly/lazytalos/internal/ui/dashboard"
	"github.com/larkly/lazytalos/internal/ui/configview"
	"github.com/larkly/lazytalos/internal/ui/etcdview"
	"github.com/larkly/lazytalos/internal/ui/help"
	"github.com/larkly/lazytalos/internal/ui/logviewer"
	"github.com/larkly/lazytalos/internal/ui/modal"
	"github.com/larkly/lazytalos/internal/ui/network"
	"github.com/larkly/lazytalos/internal/ui/nodelist"
	"github.com/larkly/lazytalos/internal/ui/servicelist"
	"github.com/larkly/lazytalos/internal/ui/settings"
	upgradeui "github.com/larkly/lazytalos/internal/ui/upgrade"
	"github.com/larkly/lazytalos/internal/ui/statusbar"
	"github.com/larkly/lazytalos/internal/ui/storage"
)

type activeView int

const (
	viewContextPicker activeView = iota
	viewDashboard
	viewNodeList
	viewServiceList
	viewLogViewer
	viewContainers
	viewNetwork
	viewStorage
	viewEtcd
)

// configFetchedMsg is an internal message carrying freshly-fetched config YAML.
type configFetchedMsg struct {
	node   string
	yaml   string
	width  int
	height int
}

type modalType int

const (
	modalNone modalType = iota
	modalConfirm
	modalError
	modalReset
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
	containers  containers.Model
	network     network.Model
	storage     storage.Model
	etcdView    etcdview.Model

	// Help overlay
	help help.Model

	// Config view overlay
	configView configview.Model

	// Settings overlay
	settings  settings.Model
	appConfig *config.Config

	// Config editor overlay
	configEditor  configeditor.Model
	editingConfig bool

	// Upgrade wizard overlay
	upgradeWizard  upgradeui.Model
	showingUpgrade bool

	// Selection (for bulk node ops)
	selectedNodes map[string]bool

	// Modals
	activeModal modalType
	confirm     modal.ConfirmModel
	errModal    modal.ErrorModel
	resetModal  modal.ResetModal

	// etcd member removal state
	etcdMemberNode string
	etcdMemberID   uint64

	// UI
	statusBar statusbar.Model
	view      activeView

	// Sizing
	width  int
	height int

	// Config
	refreshInterval     time.Duration
	talosconfig         string
	pickContext         bool // always show picker
	noUpdateCheck       bool
	updateCheckInterval time.Duration

	// State
	restart        bool
	autoSelect     string // context to auto-select on Init
	version        string
	latestVersion  string // set when a newer release is detected
}

// Options configures the application.
type Options struct {
	Talosconfig          string
	Context              string
	RefreshInterval      time.Duration
	PickContext           bool
	Version              string
	Plain                bool
	NoUpdateCheck        bool
	UpdateCheckInterval  time.Duration
	AppConfig            *config.Config
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
		version:         opts.Version,
		selectedNodes:   make(map[string]bool),
		help:            help.New(),
		configView:      configview.New(opts.Talosconfig),
		noUpdateCheck:       opts.NoUpdateCheck,
		updateCheckInterval: opts.UpdateCheckInterval,
		appConfig:           opts.AppConfig,
	}
	if m.appConfig != nil {
		m.settings = settings.New(m.appConfig)
	} else {
		defaults := config.Defaults()
		m.appConfig = &defaults
		m.settings = settings.New(m.appConfig)
	}
	if m.updateCheckInterval == 0 {
		m.updateCheckInterval = 24 * time.Hour
	}

	// Auto-select if --context flag is set, or exactly one context and not forced to pick
	if opts.Context != "" && !opts.PickContext {
		m.autoSelect = opts.Context
		m.contextPicker = contextpicker.New(contexts, m.contextName, err)
	} else if err == nil && len(contexts) == 1 && !opts.PickContext {
		m.autoSelect = contexts[0]
		m.contextPicker = contextpicker.New(contexts, m.contextName, nil)
	} else {
		m.contextPicker = contextpicker.New(contexts, m.contextName, err)
	}

	m.statusBar.CurrentView = "contextpicker"
	m.statusBar.Hint = "Select a context to connect"

	return m
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.autoSelect != "" {
		name := m.autoSelect
		cmds = append(cmds, func() tea.Msg {
			return shared.ContextSelectedMsg{ContextName: name}
		})
	}

	if !m.noUpdateCheck {
		ver := m.version
		ttl := m.updateCheckInterval
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rel, _ := update.CheckLatestCached(ctx, ver, ttl)
			if rel == nil {
				return nil
			}
			if update.IsNewer(rel.Version, ver) {
				return shared.UpdateAvailableMsg{Version: rel.Version, URL: rel.URL}
			}
			return nil
		})
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Config editor overlay: route all messages to it while active.
	if m.editingConfig {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.statusBar.Width = m.width
			m.configEditor.SetSize(m.width, m.contentHeight())
			return m, nil
		case configeditor.ClosedMsg:
			m.editingConfig = false
			if msg.Applied {
				m.statusBar.Hint = "Config applied successfully"
			}
			return m, nil
		default:
			var ceCmd tea.Cmd
			m.configEditor, ceCmd = m.configEditor.Update(msg)
			return m, ceCmd
		}
	}

	// Upgrade wizard overlay: route all messages to it while active.
	if m.showingUpgrade {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.statusBar.Width = m.width
			m.upgradeWizard.SetSize(m.width, m.contentHeight())
			return m, nil
		case upgradeui.ClosedMsg:
			m.showingUpgrade = false
			if msg.Completed {
				m.statusBar.Hint = "Cluster upgrade completed"
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.upgradeWizard, cmd = m.upgradeWizard.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.contextPicker.SetSize(m.width, m.height)
		m.confirm.SetSize(m.width, m.height)
		m.errModal.SetSize(m.width, m.height)
		m.statusBar.Width = m.width
		m.help.SetSize(m.width, m.height)
		m.configView.SetSize(m.width, m.height)
		m.settings.Width = m.width
		m.settings.Height = m.height
		// Propagate size to all initialized views
		ch := m.contentHeight()
		if m.tabInited[0] {
			m.dashboard.SetSize(m.width, ch)
		}
		if m.tabInited[1] {
			m.nodeList.SetSize(m.width, ch)
		}
		if m.tabInited[2] {
			m.serviceList.SetSize(m.width, ch)
		}
		if m.tabInited[3] {
			m.logViewer.SetSize(m.width, ch)
		}
		if m.tabInited[4] {
			m.containers.SetSize(m.width, ch)
		}
		if m.tabInited[5] {
			m.network.SetSize(m.width, ch)
		}
		if m.tabInited[6] {
			m.storage.SetSize(m.width, ch)
		}
		if m.tabInited[7] {
			m.etcdView.SetSize(m.width, ch)
		}
		return m, nil

	case tea.KeyMsg:
		// Modal intercepts all keys
		if m.activeModal != modalNone {
			return m.updateModal(msg)
		}

		// Help overlay intercepts all keys when visible
		if m.help.IsVisible() {
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}

		// Settings overlay intercepts all keys when visible
		if m.settings.Visible {
			var cmd tea.Cmd
			m.settings, cmd = m.settings.Update(msg)
			return m, cmd
		}

		// Config view overlay intercepts all keys when visible
		if m.configView.IsVisible() {
			var cmd tea.Cmd
			m.configView, cmd = m.configView.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, shared.Keys.Quit) && m.view != viewContextPicker:
			return m, tea.Quit
		case key.Matches(msg, shared.Keys.ContextPicker) && m.view != viewContextPicker:
			return m.switchToContextPicker()
		case key.Matches(msg, shared.Keys.Help) && m.view != viewContextPicker:
			m.help.Open(m.statusBar.CurrentView)
			return m, nil
		case key.Matches(msg, shared.Keys.Settings) && m.view != viewContextPicker:
			m.settings.Width = m.width
			m.settings.Height = m.height
			m.settings.Open()
			return m, nil
		case key.Matches(msg, shared.Keys.ConfigView) && m.view != viewContextPicker:
			m.configView = m.configView.Toggle()
			return m, nil
		}

		// Tab switching (only from top-level views)
		if m.isTopLevelView() {
			if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '8' {
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
		m.configView.SetActiveContext(msg.ContextName)
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
		shared.Debugf("[app] action result: %s ok=%v failures=%v", msg.Action, msg.Nodes, msg.Failures)
		if len(msg.Failures) > 0 {
			m.errModal = modal.NewError(msg.Action+" partially failed", formatPartialFailure(msg.Nodes, msg.Failures))
			m.errModal.SetSize(m.width, m.height)
			m.activeModal = modalError
			return m.forceRefreshActiveView()
		}
		m.statusBar.Hint = msg.Action + " completed"
		return m.forceRefreshActiveView()

	case shared.NodeActionErrMsg:
		shared.Debugf("[app] action error: %s: %v", msg.Action, msg.Err)
		m.errModal = modal.NewError(msg.Action+" failed", msg.Err)
		m.errModal.SetSize(m.width, m.height)
		m.activeModal = modalError
		return m, nil

	case shared.NodeActionRequestMsg:
		// Child view wants to show a confirm modal for a node action.
		shared.Debugf("[app] node action request: %s on %v", msg.Action, msg.NodeHostnames)
		nodes := make([]modal.NodeRef, len(msg.NodeHostnames))
		hasCP := false
		for i, h := range msg.NodeHostnames {
			isCP := false
			if i < len(msg.IsControlPlane) {
				isCP = msg.IsControlPlane[i]
			}
			name := h
			if i < len(msg.NodeNames) {
				name = msg.NodeNames[i]
			}
			nodes[i] = modal.NodeRef{ID: h, Name: name, IsControlPlane: isCP}
			if isCP {
				hasCP = true
			}
		}
		_ = hasCP
		if len(nodes) == 1 {
			m.confirm = modal.NewConfirm(msg.Action, nodes[0].ID)
		} else {
			m.confirm = modal.NewBulkConfirm(msg.Action, nodes)
		}
		m.confirm.SetSize(m.width, m.height)
		m.activeModal = modalConfirm
		return m, nil

	case shared.ServiceRestartRequestMsg:
		// Child view wants to show a service restart confirm modal.
		shared.Debugf("[app] service restart request: %s on %s", msg.ServiceID, msg.Node)
		m.confirm = modal.NewServiceRestartConfirm(msg.Node, msg.ServiceID, msg.ServiceID)
		m.confirm.SetSize(m.width, m.height)
		m.activeModal = modalConfirm
		return m, nil

	case shared.ViewChangeMsg:
		// Child view wants to exit (ignored at top level -- no parent to go back to)
		return m, nil

	case configview.ClosedMsg:
		// Config view was dismissed; no additional action needed.
		return m, nil

	case shared.ContainerLogsRequestMsg:
		// Switch to the Logs tab.
		m2, cmd := m.switchTab(3)
		m2.logViewer.PreSelectContainer(msg.Node, msg.ContainerID)
		return m2, cmd

	case shared.ConfigEditRequestMsg:
		// Fetch config async, then open the editor.
		shared.Debugf("[app] config edit request for node %s", msg.Node)
		node := msg.Node
		client := m.client
		w, h := m.width, m.contentHeight()
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cfg, err := config.FetchConfig(ctx, client, node)
			if err != nil {
				return shared.NodeActionErrMsg{Action: "edit config", Err: err}
			}
			return configFetchedMsg{node: node, yaml: cfg.YAML, width: w, height: h}
		}

	case configFetchedMsg:
		m.configEditor = configeditor.New(m.client, msg.node, msg.yaml, msg.width, msg.height)
		m.editingConfig = true
		return m, nil

	case shared.UpgradeRequestMsg:
		// Collect NodeInfo for the requested hostnames from the node list.
		allNodes := m.nodeList.AllNodes()
		requested := make(map[string]bool, len(msg.Nodes))
		for _, h := range msg.Nodes {
			requested[h] = true
		}
		var upgradeNodes []cluster.NodeInfo
		for _, n := range allNodes {
			if requested[n.Hostname] {
				upgradeNodes = append(upgradeNodes, n)
			}
		}
		if len(upgradeNodes) == 0 {
			return m, nil
		}
		m.upgradeWizard = upgradeui.New(m.client, m.contextName, upgradeNodes, m.width, m.contentHeight())
		m.showingUpgrade = true
		return m, nil

	case shared.EtcdMemberRemoveRequestMsg:
		shared.Debugf("[app] etcd member remove request: node=%s memberID=%x", msg.Node, msg.MemberID)
		m.etcdMemberNode = msg.Node
		m.etcdMemberID = msg.MemberID
		m.confirm = modal.NewTypedConfirm("remove etcd member", msg.Node, fmt.Sprintf("%x", msg.MemberID))
		m.confirm.SetSize(m.width, m.height)
		m.activeModal = modalConfirm
		return m, nil

	case shared.NodeResetRequestMsg:
		m.resetModal = modal.NewResetModal(msg.Node, msg.IsControlPlane, m.width, m.height)
		m.activeModal = modalReset
		return m, nil

	case modal.ResetConfirmedMsg:
		m.activeModal = modalNone
		addrs := m.resolveToAddresses([]string{msg.Node})
		target := msg.Node
		if len(addrs) > 0 {
			target = addrs[0]
		}
		return m, m.resetNode(target, msg.Graceful)

	case modal.ResetCancelledMsg:
		m.activeModal = modalNone
		return m, nil

	case shared.LogLineMsg, shared.LogStreamEndedMsg:
		// Always route log messages to the logviewer, even if it's not active.
		if m.tabInited[3] {
			var cmd tea.Cmd
			m.logViewer, cmd = m.logViewer.Update(msg)
			return m, cmd
		}
		return m, nil

	case shared.YankMsg:
		m.statusBar.Hint = "Copied: " + msg.Text
		return m, nil

	case shared.UpdateAvailableMsg:
		shared.Debugf("[app] update available: %s (%s)", msg.Version, msg.URL)
		m.latestVersion = msg.Version
		m.statusBar.Hint = "Update available: " + msg.Version + " — " + msg.URL
		return m, nil

	case shared.ConfigChangedMsg:
		shared.Debugf("[app] config changed, updating runtime settings")
		if m.appConfig != nil {
			m.refreshInterval = time.Duration(m.appConfig.General.RefreshInterval) * time.Second
			m.noUpdateCheck = !m.appConfig.General.CheckForUpdates
			m.updateCheckInterval = time.Duration(m.appConfig.General.UpdateCheckInterval) * time.Hour
			m.pickContext = m.appConfig.General.AlwaysPickContext
		}
		return m, nil

	case shared.TickMsg:
		m2, viewCmd := m.updateAllViews(msg)
		return m2, tea.Batch(viewCmd, m.refreshTickCmd())
	}

	return m.updateActiveView(msg)
}

func (m Model) handleConfirmedAction(action modal.ConfirmAction) (Model, tea.Cmd) {
	// Collect hostnames from confirmed action
	var hostnames []string
	for _, n := range action.Nodes {
		hostnames = append(hostnames, n.ID)
	}
	if len(hostnames) == 0 && action.Node != "" {
		hostnames = []string{action.Node}
	}

	// Resolve hostnames to routable addresses for WithNodes
	nodes := m.resolveToAddresses(hostnames)

	switch action.Action {
	case "reboot":
		return m, m.rebootNodes(nodes)
	case "shutdown":
		return m, m.shutdownNodes(nodes)
	case "restart service":
		if action.Node != "" && action.ServiceID != "" {
			svcAddrs := m.resolveToAddresses([]string{action.Node})
			svcTarget := action.Node
			if len(svcAddrs) > 0 {
				svcTarget = svcAddrs[0]
			}
			return m, m.restartService(svcTarget, action.ServiceID)
		}
	case "remove etcd member":
		node := m.etcdMemberNode
		memberID := m.etcdMemberID
		client := m.client
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := etcd.RemoveMemberByID(ctx, client, node, memberID)
			if err != nil {
				return shared.NodeActionErrMsg{Action: "remove etcd member", Err: err}
			}
			return shared.NodeActionMsg{Action: "remove etcd member", Nodes: []string{node}}
		}
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
	case viewContainers:
		return m, m.containers.ForceRefresh()
	case viewNetwork:
		return m, m.network.ForceRefresh()
	case viewStorage:
		return m, m.storage.ForceRefresh()
	case viewEtcd:
		return m, m.etcdView.ForceRefresh()
	}
	return m, nil
}

// syncLogViewerNodes populates the log viewer's node selector from the
// discovered cluster members, mapping hostnames to their first address
// so the UI shows friendly names while API calls use routable addresses.
func (m *Model) syncLogViewerNodes() {
	// Try the node list first (already loaded by tab 2)
	allNodes := m.nodeList.AllNodes()
	if len(allNodes) == 0 {
		// Fall back to direct member discovery
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		allNodes, _ = cluster.GetMembers(ctx, m.client)
	}
	if len(allNodes) > 0 {
		hostnames := make([]string, len(allNodes))
		addrs := make(map[string]string, len(allNodes))
		for i, n := range allNodes {
			hostnames[i] = n.Hostname
			if len(n.Addresses) > 0 {
				addrs[n.Hostname] = n.Addresses[0]
			}
		}
		m.logViewer.SetNodesWithAddrs(hostnames, addrs)
	}
}

// resolveToAddresses maps hostnames to routable addresses using the node list.
// If a hostname has no known address, it's returned as-is.
func (m Model) resolveToAddresses(hostnames []string) []string {
	allNodes := m.nodeList.AllNodes()
	addrMap := make(map[string]string, len(allNodes))
	for _, n := range allNodes {
		if len(n.Addresses) > 0 {
			addrMap[n.Hostname] = n.Addresses[0]
		}
	}
	result := make([]string, len(hostnames))
	for i, h := range hostnames {
		if addr, ok := addrMap[h]; ok {
			result[i] = addr
		} else {
			result[i] = h
		}
	}
	return result
}
