// Package config provides helpers for fetching, validating, and applying
// Talos machine configurations over the machinery gRPC API.
package config

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"gopkg.in/yaml.v3"

	"github.com/larkly/lazytalos/internal/talos"
)

// MachineConfig holds the fetched machine configuration for a single node.
type MachineConfig struct {
	Node    string
	YAML    string
	Version string
}

// ApplyMode controls how a config change is applied.
type ApplyMode string

const (
	ApplyModeReboot   ApplyMode = "reboot"
	ApplyModeNoReboot ApplyMode = "no-reboot"
	ApplyModeStaged   ApplyMode = "staged"
)

// FetchConfig retrieves the machine config YAML for a single node via the COSI
// state (namespace "config", type MachineConfigs.config.talos.dev, id "v1alpha1").
func FetchConfig(ctx context.Context, c *talos.Client, node string) (MachineConfig, error) {
	nodeCtx := talosclient.WithNodes(ctx, node)

	md := resource.NewMetadata(
		configresource.NamespaceName,
		configresource.MachineConfigType,
		configresource.ActiveID,
		resource.VersionUndefined,
	)

	raw, err := c.C.COSI.Get(nodeCtx, md)
	if err != nil {
		return MachineConfig{}, fmt.Errorf("fetch machine config for node %s: %w", node, err)
	}

	mc, ok := raw.(*configresource.MachineConfig)
	if !ok {
		return MachineConfig{}, fmt.Errorf("unexpected resource type %T for node %s", raw, node)
	}

	yamlBytes, err := mc.Provider().Bytes()
	if err != nil {
		return MachineConfig{}, fmt.Errorf("encode machine config YAML for node %s: %w", node, err)
	}

	return MachineConfig{
		Node:    node,
		YAML:    string(yamlBytes),
		Version: raw.Metadata().Version().String(),
	}, nil
}

// ValidateConfig checks the given YAML config for the node.
// It performs a local syntax check first; server-side dry-run is not
// available in the Talos API so only syntax errors are returned.
// Returns a list of human-readable validation error strings (empty = valid).
func ValidateConfig(ctx context.Context, c *talos.Client, node string, yamlStr string) ([]string, error) {
	if errs := parseSyntaxErrors(yamlStr); len(errs) > 0 {
		return errs, nil
	}
	return nil, nil
}

// parseSyntaxErrors attempts to decode yamlStr and returns a non-empty slice
// of error strings if the YAML is syntactically invalid.
// Exported as a separate helper so tests can exercise it without a live client.
func parseSyntaxErrors(yamlStr string) []string {
	var out interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// applyModeToProto maps our ApplyMode constants to the machinery protobuf enum.
func applyModeToProto(mode ApplyMode) (machine.ApplyConfigurationRequest_Mode, error) {
	switch mode {
	case ApplyModeReboot:
		return machine.ApplyConfigurationRequest_REBOOT, nil
	case ApplyModeNoReboot:
		return machine.ApplyConfigurationRequest_NO_REBOOT, nil
	case ApplyModeStaged:
		return machine.ApplyConfigurationRequest_STAGED, nil
	default:
		return 0, fmt.Errorf("unknown apply mode: %q", mode)
	}
}

// ApplyConfig applies the given YAML config to the node with the specified mode.
func ApplyConfig(ctx context.Context, c *talos.Client, node string, yamlStr string, mode ApplyMode) error {
	protoMode, err := applyModeToProto(mode)
	if err != nil {
		return err
	}

	nodeCtx := talosclient.WithNodes(ctx, node)

	_, err = c.C.ApplyConfiguration(nodeCtx, &machine.ApplyConfigurationRequest{
		Data: []byte(yamlStr),
		Mode: protoMode,
	})
	if err != nil {
		return fmt.Errorf("apply config to node %s: %w", node, err)
	}

	return nil
}
