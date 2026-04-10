package config

import (
	"charm.land/lipgloss/v2"

	"github.com/larkly/lazytalos/internal/shared"
)

// ApplyAll applies all config sections to the shared runtime state.
func ApplyAll(cfg Config) {
	ApplyGeneral(cfg.General)
	ApplyColors(cfg.Colors)
	ApplyThresholds(cfg.Thresholds)
}

// ApplyGeneral sets shared.PlainMode from config.
func ApplyGeneral(g GeneralConfig) {
	shared.PlainMode = g.PlainMode
}

// ApplyColors sets shared.Color* vars and rebuilds shared.Style* vars.
func ApplyColors(c ColorConfig) {
	if c.Primary != "" {
		shared.ColorPrimary = lipgloss.Color(c.Primary)
	}
	if c.Secondary != "" {
		shared.ColorSecondary = lipgloss.Color(c.Secondary)
	}
	if c.Success != "" {
		shared.ColorSuccess = lipgloss.Color(c.Success)
	}
	if c.Warning != "" {
		shared.ColorWarning = lipgloss.Color(c.Warning)
	}
	if c.Error != "" {
		shared.ColorError = lipgloss.Color(c.Error)
	}
	if c.Muted != "" {
		shared.ColorMuted = lipgloss.Color(c.Muted)
	}
	if c.Bg != "" {
		shared.ColorBg = lipgloss.Color(c.Bg)
	}
	if c.Fg != "" {
		shared.ColorFg = lipgloss.Color(c.Fg)
	}
	if c.Highlight != "" {
		shared.ColorHighlight = lipgloss.Color(c.Highlight)
	}
	shared.RebuildStyles()
}

// ApplyThresholds sets shared threshold vars from config.
func ApplyThresholds(t ThresholdConfig) {
	if t.MemoryWarning > 0 {
		shared.MemWarningPct = float64(t.MemoryWarning) / 100.0
	}
	if t.MemoryCritical > 0 {
		shared.MemCriticalPct = float64(t.MemoryCritical) / 100.0
	}
	if t.CPUWarning > 0 {
		shared.CPUWarningPct = float64(t.CPUWarning) / 100.0
	}
}
