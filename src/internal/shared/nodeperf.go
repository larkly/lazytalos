package shared

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Configurable threshold vars for resource bar coloring.
var (
	MemWarningPct  = 0.6
	MemCriticalPct = 0.8
	CPUWarningPct  = 0.7
)

// MemStats holds memory usage for a single node.
type MemStats struct {
	NodeHostname string
	TotalKB      uint64
	AvailableKB  uint64
}

// RenderMemBar creates a block-character memory bar like "62% ████████░░░░".
func RenderMemBar(pct float64, width int) string {
	if width < 4 {
		width = 4
	}
	pctStr := fmt.Sprintf("%3.0f%%", pct*100)
	barW := width - 5 // space for " 62% "
	if barW < 1 {
		barW = 1
	}
	filled := int(pct * float64(barW))
	if filled > barW {
		filled = barW
	}
	empty := barW - filled

	barStyle := StyleSuccess
	if pct > MemCriticalPct {
		barStyle = StyleError
	} else if pct > MemWarningPct {
		barStyle = StyleWarning
	}

	bar := barStyle.Render(strings.Repeat("█", filled)) +
		StyleMuted.Render(strings.Repeat("░", empty))
	return fmt.Sprintf("%s %s", pctStr, bar)
}

// RenderCPUBar creates a block-character CPU bar like " 45% ████████░░░░".
func RenderCPUBar(pct float64, width int) string {
	if width < 4 {
		width = 4
	}
	pctStr := fmt.Sprintf("%3.0f%%", pct*100)
	barW := width - 5
	if barW < 1 {
		barW = 1
	}
	filled := int(pct * float64(barW))
	if filled > barW {
		filled = barW
	}
	empty := barW - filled

	barStyle := StyleSuccess
	if pct > CPUWarningPct {
		barStyle = StyleWarning
	}

	bar := barStyle.Render(strings.Repeat("█", filled)) +
		StyleMuted.Render(strings.Repeat("░", empty))
	return fmt.Sprintf("%s %s", pctStr, bar)
}

// MemUsedPct computes memory usage as a fraction in [0,1]. Returns 0 when
// TotalKB is zero (stats unknown) so callers can treat it as "no signal".
func MemUsedPct(m MemStats) float64 {
	if m.TotalKB == 0 {
		return 0
	}
	return float64(m.TotalKB-m.AvailableKB) / float64(m.TotalKB)
}

// NodeDotStyle picks the icon and lipgloss style for a single node-health dot
// using the same four-state logic shared by the dashboard node matrix and the
// multi-cluster grid cards.
//
//   - hasSvcs:    service data was returned for this node.
//   - hasAnyData: service data was returned for AT LEAST ONE node in the
//     cluster. Together with !hasSvcs this distinguishes "this node is
//     unreachable" from "services were never fetched for the cluster".
//   - failedSvc:  any service on this node has Health == Failed.
//   - memPct:     memory usage fraction 0..1, or 0 when unknown.
//   - cpuPct:     CPU usage fraction 0..1, or 0 when unknown.
//
// Thresholds are MemCriticalPct / MemWarningPct / CPUWarningPct.
func NodeDotStyle(hasSvcs, hasAnyData, failedSvc bool, memPct, cpuPct float64) (string, lipgloss.Style) {
	if !hasSvcs && hasAnyData {
		return "○", StyleMuted
	}
	if failedSvc {
		return "●", StyleError
	}
	if memPct > MemCriticalPct {
		return "●", StyleError
	}
	if cpuPct > CPUWarningPct || memPct > MemWarningPct {
		return "●", StyleWarning
	}
	return "●", StyleSuccess
}

// FormatUptime formats a duration as a compact uptime string like "2d3h" or "1h20m".
func FormatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}
