// Package nodelist provides the cluster nodes tab view.
// Stub -- will be implemented in Task 10.
package nodelist

import (
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/talos"
)

// Model is the node list view model.
type Model struct {
	client          *talos.Client
	refreshInterval time.Duration
	width           int
	height          int
}

// New creates a new node list model.
func New(client *talos.Client, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		refreshInterval: refreshInterval,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) { return m, nil }

// View renders the node list.
func (m Model) View() string { return "  Nodes (not yet implemented)" }

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Hints returns status bar hint text.
func (m Model) Hints() string { return "" }

// ForceRefresh triggers an immediate data refresh.
func (m Model) ForceRefresh() tea.Cmd { return nil }
