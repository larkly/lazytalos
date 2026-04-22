package talos

import (
	"fmt"
	"os"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/client/config"
)

// ListContextNames returns the list of context names from a talosconfig file,
// plus the name of the currently active context.
// Returns (contextNames []string, currentContext string, err error).
//
// When talosconfig is a non-empty path that does not exist, an error is
// returned instead of silently creating an empty file there (which is the
// default behaviour of config.Open).
func ListContextNames(talosconfig string) ([]string, string, error) {
	if talosconfig != "" {
		if _, err := os.Stat(talosconfig); err != nil {
			return nil, "", fmt.Errorf("talosconfig %q: %w", talosconfig, err)
		}
	}

	cfg, err := config.Open(talosconfig)
	if err != nil {
		return nil, "", err
	}

	names := make([]string, 0, len(cfg.Contexts))
	for name := range cfg.Contexts {
		names = append(names, name)
	}
	slices.Sort(names)

	return names, cfg.Context, nil
}
