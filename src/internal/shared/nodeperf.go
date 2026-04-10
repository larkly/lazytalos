package shared

import (
	"fmt"
	"strings"
	"time"
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
