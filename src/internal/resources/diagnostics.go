package resources

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"

	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// DiagnosticEntry holds a single diagnostic warning from a Talos node.
type DiagnosticEntry struct {
	NodeHostname string
	ID           string
	Severity     string // "warning" (Talos diagnostics are warnings by nature)
	Message      string
	Details      string
}

// ListDiagnostics returns all Diagnostic resources across all nodes.
// Talos diagnostics are runtime warnings (e.g. address overlap, config issues).
func ListDiagnostics(ctx context.Context, c *talos.Client) ([]DiagnosticEntry, error) {
	md := resource.NewMetadata(
		runtimeres.NamespaceName,
		runtimeres.DiagnosticType,
		"",
		resource.VersionUndefined,
	)

	var results []DiagnosticEntry

	err := listPerNode(ctx, c, md, func(nodeAddr string, item resource.Resource) {
		r, ok := item.(*runtimeres.Diagnostic)
		if !ok {
			shared.Debugf("[resources] unexpected Diagnostic type: %T", item)
			return
		}

		spec := r.TypedSpec()
		details := ""
		if len(spec.Details) > 0 {
			details = spec.Details[0]
		}

		results = append(results, DiagnosticEntry{
			NodeHostname: nodeAddr,
			ID:           item.Metadata().ID(),
			Severity:     "warning",
			Message:      spec.Message,
			Details:      details,
		})
	})

	// Diagnostics may not be present on all nodes; treat errors gracefully.
	if err != nil {
		shared.Debugf("[resources] Diagnostic list error: %v", err)
		return nil, nil
	}

	return results, nil
}
