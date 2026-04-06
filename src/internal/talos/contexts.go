package talos

import (
	"sort"

	"github.com/siderolabs/talos/pkg/machinery/client/config"
)

// ListContextNames returns the list of context names from a talosconfig file,
// plus the name of the currently active context.
// Returns (contextNames []string, currentContext string, err error).
func ListContextNames(talosconfig string) ([]string, string, error) {
	cfg, err := config.Open(talosconfig)
	if err != nil {
		return nil, "", err
	}

	names := make([]string, 0, len(cfg.Contexts))
	for name := range cfg.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, cfg.Context, nil
}
