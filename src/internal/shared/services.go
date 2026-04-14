package shared

import machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"

// Canonical service health strings used across UI and aggregation code.
// These are the values assigned to ServiceRow.Health.
const (
	HealthOK      = "OK"
	HealthFailed  = "Failed"
	HealthUnknown = "?"
)

// ServiceRow is a compact view of a Talos machine service suitable for
// rendering in health matrices and dot-matrix overviews.
type ServiceRow struct {
	ServiceID string
	State     string
	Health    string
}

// ClassifyServiceHealth maps a Talos ServiceHealth proto to one of the
// canonical Health* strings.
func ClassifyServiceHealth(h *machineapi.ServiceHealth) string {
	if h == nil || h.GetUnknown() {
		return HealthUnknown
	}
	if h.GetHealthy() {
		return HealthOK
	}
	return HealthFailed
}
