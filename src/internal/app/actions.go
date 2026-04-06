package app

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
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

// nodeContext creates a context with node metadata set for the Talos client.
func nodeContext(ctx context.Context, node string) context.Context {
	// The Talos machinery client uses gRPC metadata to target specific nodes.
	// We use the client's WithNode helper via context metadata.
	md := map[string]string{"nodes": node}
	return contextWithMetadata(ctx, md)
}

// contextWithMetadata adds gRPC metadata to a context for Talos client calls.
func contextWithMetadata(ctx context.Context, md map[string]string) context.Context {
	// Use the google.golang.org/grpc/metadata package to add node targeting.
	// The Talos client expects "nodes" key in outgoing gRPC metadata.
	return ctx // stub: will be implemented with proper gRPC metadata in later tasks
}
