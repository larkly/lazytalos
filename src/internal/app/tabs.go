package app

import (
	"fmt"
	"strings"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/ui/containers"
	"github.com/larkly/lazytalos/internal/ui/dashboard"
	"github.com/larkly/lazytalos/internal/ui/etcdview"
	"github.com/larkly/lazytalos/internal/ui/logviewer"
	"github.com/larkly/lazytalos/internal/ui/networkview"
	"github.com/larkly/lazytalos/internal/ui/nodelist"
	"github.com/larkly/lazytalos/internal/ui/servicelist"
	"github.com/larkly/lazytalos/internal/ui/storageview"
)

// TabDef describes a resource tab.
type TabDef struct {
	Name string // tab bar label
	Key  string // identifier
}

// defaultTabs returns the default set of resource tabs.
func defaultTabs() []TabDef {
	return []TabDef{
		{Name: "Dashboard", Key: "dashboard"},
		{Name: "Nodes", Key: "nodes"},
		{Name: "Services", Key: "services"},
		{Name: "Logs", Key: "logs"},
		{Name: "Containers", Key: "containers"},
		{Name: "Network", Key: "network"},
		{Name: "Storage", Key: "storage"},
		{Name: "etcd", Key: "etcd"},
	}
}

func (m Model) isTopLevelView() bool {
	switch m.view {
	case viewDashboard, viewNodeList, viewServiceList, viewLogViewer,
		viewContainers, viewNetwork, viewStorage, viewEtcd:
		return true
	}
	return false
}

func (m Model) switchTab(idx int) (Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.tabs) {
		return m, nil
	}
	if idx == m.activeTab && m.isTopLevelView() {
		return m, nil
	}
	m.activeTab = idx
	td := m.tabs[idx]

	switch td.Key {
	case "dashboard":
		m.view = viewDashboard
		m.statusBar.CurrentView = "dashboard"
		if !m.tabInited[idx] {
			m.dashboard = dashboard.New(m.client, m.refreshInterval)
			m.dashboard.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.dashboard.Hints()
			return m, m.dashboard.Init()
		}
		m.statusBar.Hint = m.dashboard.Hints()
		return m, m.dashboard.ForceRefresh()

	case "nodes":
		m.view = viewNodeList
		m.statusBar.CurrentView = "nodes"
		if !m.tabInited[idx] {
			m.nodeList = nodelist.New(m.client, m.refreshInterval)
			m.nodeList.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.nodeList.Hints()
			return m, m.nodeList.Init()
		}
		m.statusBar.Hint = m.nodeList.Hints()
		return m, m.nodeList.ForceRefresh()

	case "services":
		m.view = viewServiceList
		m.statusBar.CurrentView = "services"
		if !m.tabInited[idx] {
			m.serviceList = servicelist.New(m.client, m.refreshInterval)
			m.serviceList.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.serviceList.Hints()
			return m, m.serviceList.Init()
		}
		m.statusBar.Hint = m.serviceList.Hints()
		return m, m.serviceList.ForceRefresh()

	case "logs":
		m.view = viewLogViewer
		m.statusBar.CurrentView = "logs"
		if !m.tabInited[idx] {
			m.logViewer = logviewer.New(m.client, m.refreshInterval)
			m.logViewer.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.logViewer.Hints()
			return m, m.logViewer.Init()
		}
		m.statusBar.Hint = m.logViewer.Hints()
		return m, m.logViewer.ForceRefresh()

	case "containers":
		m.view = viewContainers
		m.statusBar.CurrentView = "containers"
		if !m.tabInited[idx] {
			m.containers = containers.New(m.client, m.refreshInterval)
			m.containers.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.containers.Hints()
			return m, m.containers.Init()
		}
		m.statusBar.Hint = m.containers.Hints()
		return m, m.containers.ForceRefresh()

	case "network":
		m.view = viewNetwork
		m.statusBar.CurrentView = "network"
		if !m.tabInited[idx] {
			m.network = networkview.New(m.client, m.refreshInterval)
			m.network.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.network.Hints()
			return m, m.network.Init()
		}
		m.statusBar.Hint = m.network.Hints()
		return m, m.network.ForceRefresh()

	case "storage":
		m.view = viewStorage
		m.statusBar.CurrentView = "storage"
		if !m.tabInited[idx] {
			m.storage = storageview.New(m.client, m.refreshInterval)
			m.storage.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.storage.Hints()
			return m, m.storage.Init()
		}
		m.statusBar.Hint = m.storage.Hints()
		return m, m.storage.ForceRefresh()

	case "etcd":
		m.view = viewEtcd
		m.statusBar.CurrentView = "etcd"
		if !m.tabInited[idx] {
			m.etcd = etcdview.New(m.client, m.refreshInterval)
			m.etcd.SetSize(m.width, m.contentHeight())
			m.tabInited[idx] = true
			m.statusBar.Hint = m.etcd.Hints()
			return m, m.etcd.Init()
		}
		m.statusBar.Hint = m.etcd.Hints()
		return m, m.etcd.ForceRefresh()
	}
	return m, nil
}

// contentHeight returns the height available for tab content (minus tab bar and status bar).
func (m Model) contentHeight() int {
	h := m.height - 2 // tab bar + status bar
	if h < 0 {
		return 0
	}
	return h
}

func (m Model) renderTabBar() string {
	var tabs []string
	for i, td := range m.tabs {
		label := fmt.Sprintf(" %d:%s ", i+1, td.Name)
		if i == m.activeTab {
			tabs = append(tabs, lipgloss.NewStyle().
				Background(shared.ColorPrimary).
				Foreground(shared.ColorBg).
				Bold(true).
				Render(label))
		} else {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(shared.ColorMuted).
				Render(label))
		}
	}
	return strings.Join(tabs, lipgloss.NewStyle().Foreground(shared.ColorMuted).Render("|"))
}
