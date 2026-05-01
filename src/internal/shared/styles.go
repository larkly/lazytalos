package shared

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Color palette (Solarized Dark).
var (
	ColorPrimary     color.Color
	ColorSecondary   color.Color
	ColorSuccess     color.Color
	ColorWarning     color.Color
	ColorError       color.Color
	ColorMuted       color.Color
	ColorBg          color.Color
	ColorFg          color.Color
	ColorHighlight   color.Color
	ColorSelection   color.Color // accent for selected rows/cards (borders)
	ColorSelectionBg color.Color // dimmed variant for selected row backgrounds

	// NodeColors provides 6 distinct colors for log viewer node identification.
	NodeColors []color.Color

	// plainMode disables Unicode icons when true (set via --plain flag).
	// Use IsPlainMode() / SetPlainMode() to read/write.
	plainMode bool
)

// Styles (Lip Gloss v2).
var (
	StyleTitle        lipgloss.Style
	StyleStatusBar    lipgloss.Style
	StyleHeader       lipgloss.Style
	StyleSelected     lipgloss.Style
	StyleMuted        lipgloss.Style
	StyleSuccess      lipgloss.Style
	StyleWarning      lipgloss.Style
	StyleError        lipgloss.Style
	StyleModal        lipgloss.Style
	StyleModalTitle   lipgloss.Style
	StyleErrorModal   lipgloss.Style
	StyleButton       lipgloss.Style
	StyleButtonSubmit lipgloss.Style
	StyleButtonCancel lipgloss.Style
	StyleLabel        lipgloss.Style
	StyleValue        lipgloss.Style
	StyleTabActive    lipgloss.Style
	StyleTabInactive  lipgloss.Style
)

func init() {
	// Initialize colors.
	ColorPrimary = lipgloss.Color("#00BCD4")
	ColorSecondary = lipgloss.Color("#56B6C2")
	ColorSuccess = lipgloss.Color("#2AA198")
	ColorWarning = lipgloss.Color("#B58900")
	ColorError = lipgloss.Color("#DC322F")
	ColorMuted = lipgloss.Color("#657B83")
	ColorBg = lipgloss.Color("#002B36")
	ColorFg = lipgloss.Color("#839496")
	ColorHighlight = lipgloss.Color("#FDF6E3")
	ColorSelection = lipgloss.Color("#268BD2")
	ColorSelectionBg = lipgloss.Color("#1A4F70")
	NodeColors = []color.Color{
		lipgloss.Color("#268BD2"),
		lipgloss.Color("#2AA198"),
		lipgloss.Color("#859900"),
		lipgloss.Color("#B58900"),
		lipgloss.Color("#CB4B16"),
		lipgloss.Color("#0097A7"),
	}
	RebuildStyles()
}

// RebuildStyles reassigns all Style* vars from current Color* values.
func RebuildStyles() {
	StyleTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		PaddingLeft(1)

	StyleStatusBar = lipgloss.NewStyle().
		Background(lipgloss.Color("#073642")).
		Foreground(ColorFg).
		PaddingLeft(1).
		PaddingRight(1)

	StyleHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary)

	StyleSelected = lipgloss.NewStyle().
		Background(ColorSelectionBg).
		Foreground(ColorHighlight).
		Bold(true)

	StyleMuted = lipgloss.NewStyle().
		Foreground(ColorMuted)

	StyleSuccess = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	StyleWarning = lipgloss.NewStyle().
		Foreground(ColorWarning)

	StyleError = lipgloss.NewStyle().
		Foreground(ColorError)

	StyleModal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	StyleModalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	StyleErrorModal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorError).
		Padding(1, 2)

	StyleButton = lipgloss.NewStyle().
		Padding(0, 2).
		Background(lipgloss.Color("#073642")).
		Foreground(ColorFg)

	StyleButtonSubmit = StyleButton.
		Background(ColorSuccess).
		Foreground(ColorBg).
		Bold(true)

	StyleButtonCancel = StyleButton.
		Background(ColorError).
		Foreground(ColorBg).
		Bold(true)

	StyleLabel = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Width(20)

	StyleValue = lipgloss.NewStyle().
		Foreground(ColorFg)

	StyleTabActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		BorderBottom(true).
		BorderForeground(ColorPrimary)

	StyleTabInactive = lipgloss.NewStyle().
		Foreground(ColorMuted)
}

// statusIconMap maps Talos status strings to their Unicode icon.
var statusIconMap = map[string]string{
	"Running":  "●",
	"OK":       "●",
	"Stopped":  "○",
	"Failed":   "✘",
	"Degraded": "▲",
	"":         "?",
}

// IsPlainMode returns whether plain (ASCII-only) mode is active.
func IsPlainMode() bool {
	return plainMode
}

// SetPlainMode enables or disables plain (ASCII-only) mode.
func SetPlainMode(v bool) {
	plainMode = v
}

// StatusIcon returns a Unicode status indicator for the given status string.
// Returns "" if plain mode is active.
func StatusIcon(status string) string {
	if plainMode {
		return ""
	}
	if icon, ok := statusIconMap[status]; ok {
		return icon
	}
	return "?"
}
