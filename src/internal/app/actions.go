package app

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
)

func (m Model) rebootNodes(nodes []string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		for _, node := range nodes {
			nodeCtx := nodeContext(ctx, node)
			_, err := client.C.MachineClient.Reboot(nodeCtx, &machine.RebootRequest{})
			if err != nil {
				return shared.NodeActionErrMsg{
					Action: "reboot",
					Err:    fmt.Errorf("reboot node %s: %w", node, err),
				}
			}
		}
		return shared.NodeActionMsg{Action: "reboot", Nodes: nodes}
	}
}

func (m Model) shutdownNodes(nodes []string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		for _, node := range nodes {
			nodeCtx := nodeContext(ctx, node)
			_, err := client.C.MachineClient.Shutdown(nodeCtx, &machine.ShutdownRequest{})
			if err != nil {
				return shared.NodeActionErrMsg{
					Action: "shutdown",
					Err:    fmt.Errorf("shutdown node %s: %w", node, err),
				}
			}
		}
		return shared.NodeActionMsg{Action: "shutdown", Nodes: nodes}
	}
}

func (m Model) restartService(node, serviceID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		nodeCtx := nodeContext(ctx, node)
		_, err := client.C.MachineClient.ServiceRestart(nodeCtx, &machine.ServiceRestartRequest{
			Id: serviceID,
		})
		if err != nil {
			return shared.NodeActionErrMsg{
				Action: "restart service",
				Err:    fmt.Errorf("restart service %s on %s: %w", serviceID, node, err),
			}
		}
		return shared.NodeActionMsg{
			Action: "restart service",
			Nodes:  []string{node},
		}
	}
}

func (m Model) resetNode(node string, graceful bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		nodeCtx := talosclient.WithNodes(ctx, node)
		if err := client.C.Reset(nodeCtx, graceful, true); err != nil {
			return shared.NodeActionErrMsg{Action: "reset", Err: err}
		}
		return shared.NodeActionMsg{Action: "reset", Nodes: []string{node}}
	}
}

// nodeContext returns a context targeting a specific Talos node.
func nodeContext(ctx context.Context, node string) context.Context {
	return talosclient.WithNodes(ctx, node)
}
