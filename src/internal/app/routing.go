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
		return m, cmd
	}

	if m.tabInited[0] {
		m.dashboard, cmd = m.dashboard.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.tabInited[1] {
		m.nodeList, cmd = m.nodeList.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.tabInited[2] {
		m.serviceList, cmd = m.serviceList.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.tabInited[3] {
		// Note: logViewer is stream-driven, not poll-driven.
		// Still route TickMsg to it so it can handle window-resize and other
		// non-tick messages dispatched via updateAllViews.
		m.logViewer, cmd = m.logViewer.Update(msg)
		cmds = append(cmds, cmd)
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
