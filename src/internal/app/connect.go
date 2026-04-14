package app

import (
	"context"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
	"github.com/larkly/lazytalos/internal/ui/contextpicker"
)

// connectTimeout bounds the initial gRPC dial + TLS handshake. Without it a
// wedged endpoint (or slow DNS) would block the connect Cmd indefinitely
// with no way for the user to recover short of killing the process.
const connectTimeout = 15 * time.Second

func (m Model) connectToContext(contextName string) tea.Cmd {
	shared.Debugf("[app] connectToContext: start context=%s", contextName)
	talosconfig := m.talosconfig
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		client, err := talos.Connect(ctx, talosconfig, contextName)
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

// drillDownToContext tears down any active client/streams and dispatches
// a connect command for `name`. The existing ClientConnectedMsg handler
// will land on the Dashboard tab and start the refresh tick.
func (m Model) drillDownToContext(name string) (Model, tea.Cmd) {
	shared.Debugf("[app] drillDownToContext: %s", name)
	m.logViewer.CleanupStreams()
	m.nodeList.CancelDetailStreams()
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}
	m.tabInited = make([]bool, len(m.tabs))
	m.contextName = name
	m.statusBar.Context = name
	m.statusBar.Connected = false
	m.statusBar.Hint = "Connecting to " + name + "..."
	return m, m.connectToContext(name)
}

func (m Model) switchToContextPicker() (Model, tea.Cmd) {
	contexts, _, err := talos.ListContextNames(m.talosconfig)
	if err != nil {
		shared.Debugf("[app] switchToContextPicker: error listing contexts: %v", err)
	} else {
		shared.Debugf("[app] switchToContextPicker: found %d contexts", len(contexts))
	}
	m.contextPicker = contextpicker.New(contexts, m.contextName, err)
	m.contextPicker.SetSize(m.width, m.height)
	m.view = viewContextPicker
	m.autoSelect = ""
	m.statusBar.CurrentView = "contextpicker"
	m.statusBar.Hint = "Select a context to connect"
	m.statusBar.Connected = false

	// Clean up log streams before closing client to prevent goroutine leaks.
	// Both the dedicated log viewer and the nodelist detail view can hold
	// live streams against the current client.
	m.logViewer.CleanupStreams()
	m.nodeList.CancelDetailStreams()

	// Close existing client if any
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}

	// Reset tab init state
	m.tabInited = make([]bool, len(m.tabs))

	return m, nil
}
