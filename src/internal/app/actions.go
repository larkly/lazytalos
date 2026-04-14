package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
)

// applyPerNode runs fn for each node and partitions the results into the
// list of succeeded nodes and a map of per-node failures. Unlike a naive
// loop, it never aborts on the first error — every node gets a turn. The
// succeeded slice preserves input order; the failures map is order-free.
func applyPerNode(nodes []string, fn func(string) error) ([]string, map[string]error) {
	var succeeded []string
	var failures map[string]error
	for _, node := range nodes {
		if err := fn(node); err != nil {
			if failures == nil {
				failures = make(map[string]error, len(nodes))
			}
			failures[node] = err
			continue
		}
		succeeded = append(succeeded, node)
	}
	return succeeded, failures
}

// formatPartialFailure builds a user-facing error listing per-node failures
// (and, when useful, the successes alongside them). Keys are sorted so the
// output is deterministic.
func formatPartialFailure(succeeded []string, failures map[string]error) error {
	if len(failures) == 0 {
		return nil
	}
	var sb strings.Builder
	keys := make([]string, 0, len(failures))
	for k := range failures {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s: %v\n", k, failures[k])
	}
	if len(succeeded) > 0 {
		fmt.Fprintf(&sb, "(succeeded: %s)", strings.Join(succeeded, ", "))
	}
	return errors.New(strings.TrimRight(sb.String(), "\n"))
}

func (m Model) rebootNodes(nodes []string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		succeeded, failures := applyPerNode(nodes, func(node string) error {
			nodeCtx := nodeContext(ctx, node)
			_, err := client.C.MachineClient.Reboot(nodeCtx, &machine.RebootRequest{})
			return err
		})
		return shared.NodeActionMsg{
			Action:   "reboot",
			Nodes:    succeeded,
			Failures: failures,
		}
	}
}

func (m Model) shutdownNodes(nodes []string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		succeeded, failures := applyPerNode(nodes, func(node string) error {
			nodeCtx := nodeContext(ctx, node)
			_, err := client.C.MachineClient.Shutdown(nodeCtx, &machine.ShutdownRequest{})
			return err
		})
		return shared.NodeActionMsg{
			Action:   "shutdown",
			Nodes:    succeeded,
			Failures: failures,
		}
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
