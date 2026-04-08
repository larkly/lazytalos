package app

import (
	"strings"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/larkly/lazytalos/internal/shared"
)

// View renders the full UI.
func (m Model) View() tea.View {
	v := tea.NewView(m.viewContent())
	v.AltScreen = true
	return v
}

func (m Model) viewContent() string {
	// Upgrade wizard overlay has highest priority.
	if m.showingUpgrade {
		return m.upgradeWizard.View()
	}

	// Config editor overlay has second highest priority.
	if m.editingConfig {
		return m.configEditor.View()
	}

	// Help overlay: second highest priority.
	if m.help.IsVisible() {
		return m.help.View()
	}

	// Config view overlay: third highest priority.
	if m.configView.IsVisible() {
		return m.configView.View()
	}

	// Modal overlay priority chain
	if m.activeModal == modalConfirm {
		return m.confirm.View()
	}
	if m.activeModal == modalError {
		return m.errModal.View()
	}
	if m.activeModal == modalReset {
		return m.resetModal.View()
	}

	var content string
	switch m.view {
	case viewContextPicker:
		if m.autoSelect != "" {
			msg := shared.StyleModalTitle.Render("Connecting to " + m.autoSelect + "...")
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
		}
		return m.contextPicker.View()
	case viewDashboard:
		content = m.dashboard.View()
	case viewNodeList:
		content = m.nodeList.View()
	case viewServiceList:
		content = m.serviceList.View()
	case viewLogViewer:
		content = m.logViewer.View()
	case viewContainers:
		content = m.containers.View()
	case viewNetwork:
		content = m.network.View()
	case viewStorage:
		content = m.storage.View()
	case viewEtcd:
		content = m.etcdView.View()
	}

	// Add tab bar for top-level views
	if m.isTopLevelView() {
		content = m.renderTabBar() + "\n" + content
	}

	// Overlay app name + version on top-right
	appName := lipgloss.NewStyle().
		Foreground(shared.ColorBg).
		Background(shared.ColorPrimary).
		Bold(true).
		Padding(0, 1).
		Render("LAZYTALOS")
	versionStr := ""
	if m.version != "" {
		versionStr = lipgloss.NewStyle().Foreground(shared.ColorMuted).Render(m.version)
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		firstLine := lines[0]
		firstW := lipgloss.Width(firstLine)
		nameW := lipgloss.Width(appName)
		pad := m.width - firstW - nameW
		if pad > 0 {
			lines[0] = firstLine + strings.Repeat(" ", pad) + appName
		}
	}
	if len(lines) > 1 && versionStr != "" {
		secondLine := lines[1]
		secondW := lipgloss.Width(secondLine)
		verW := lipgloss.Width(versionStr)
		pad := m.width - secondW - verW
		if pad > 0 {
			lines[1] = secondLine + strings.Repeat(" ", pad) + versionStr
		}
	}
	content = strings.Join(lines, "\n")

	contentHeight := m.height - 1
	if contentHeight < 0 {
		contentHeight = 0
	}

	padded := lipgloss.NewStyle().Height(contentHeight).MaxHeight(contentHeight).Render(content)
	return padded + "\n" + m.statusBar.Render()
}
