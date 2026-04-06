package app

import (
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// refreshTickCmd returns a single tea.Tick that fires shared.TickMsg after
// the refresh interval. This is the ONLY tick source in the app -- views
// must not create their own tick timers.
func (m Model) refreshTickCmd() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return shared.TickMsg{}
	})
}

func (m Model) updateActiveView(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.view {
	case viewContextPicker:
		m.contextPicker, cmd = m.contextPicker.Update(msg)
	case viewDashboard:
		m.dashboard, cmd = m.dashboard.Update(msg)
		m.statusBar.Hint = m.dashboard.Hints()
	case viewNodeList:
		m.nodeList, cmd = m.nodeList.Update(msg)
		m.statusBar.Hint = m.nodeList.Hints()
	case viewServiceList:
		m.serviceList, cmd = m.serviceList.Update(msg)
		m.statusBar.Hint = m.serviceList.Hints()
	case viewLogViewer:
		m.logViewer, cmd = m.logViewer.Update(msg)
		m.statusBar.Hint = m.logViewer.Hints()
	}
	return m, cmd
}

// updateAllViews routes non-key messages to all initialized views so
// background auto-refresh ticks keep firing even when a view isn't active.
func (m Model) updateAllViews(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	if m.view == viewContextPicker {
		m.contextPicker, cmd = m.contextPicker.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Route to active tab's view
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) && m.tabInited[m.activeTab] {
		switch m.tabs[m.activeTab].Key {
		case "dashboard":
			m.dashboard, cmd = m.dashboard.Update(msg)
			cmds = append(cmds, cmd)
		case "nodes":
			m.nodeList, cmd = m.nodeList.Update(msg)
			cmds = append(cmds, cmd)
		case "services":
			m.serviceList, cmd = m.serviceList.Update(msg)
			cmds = append(cmds, cmd)
		case "logs":
			m.logViewer, cmd = m.logViewer.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateModal(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeModal {
	case modalConfirm:
		m.confirm, cmd = m.confirm.Update(msg)
	case modalError:
		m.errModal, cmd = m.errModal.Update(msg)
	}
	return m, cmd
}
