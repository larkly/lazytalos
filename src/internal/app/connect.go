package app

import (
	"context"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/larkly/lazytalos/internal/ui/contextpicker"
)

func (m Model) connectToContext(contextName string) tea.Cmd {
	shared.Debugf("[app] connectToContext: start context=%s", contextName)
	talosconfig := m.talosconfig
	return func() tea.Msg {
		client, err := talos.Connect(context.Background(), talosconfig, contextName)
		if err != nil {
			shared.Debugf("[app] connectToContext: error: %v", err)
			return shared.ClientConnectErrMsg{Err: err}
		}
		shared.Debugf("[app] connectToContext: success context=%s", contextName)
		return shared.ClientConnectedMsg{
			Client:      client,
			ContextName: contextName,
		}
	}
}

func (m Model) switchToContextPicker() (Model, tea.Cmd) {
	contexts, _, err := talos.ListContextNames(m.talosconfig)
	if err != nil {
		shared.Debugf("[app] switchToContextPicker: error listing contexts: %v", err)
	} else {
		shared.Debugf("[app] switchToContextPicker: found %d contexts", len(contexts))
	}
	m.contextPicker = contextpicker.New(contexts, err)
	m.contextPicker.SetSize(m.width, m.height)
	m.view = viewContextPicker
	m.statusBar.CurrentView = "contextpicker"
	m.statusBar.Hint = "Select a context to connect"
	m.statusBar.Connected = false

	// Clean up log streams before closing client to prevent goroutine leaks
	m.logViewer.CleanupStreams()

	// Close existing client if any
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}

	// Reset tab init state
	m.tabInited = make([]bool, len(m.tabs))

	return m, nil
}
