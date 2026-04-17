package clustergrid

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/larkly/lazytalos/internal/shared"
)

// View renders the full overlay (header + wrapped card grid).
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := m.renderHeader()

	if len(m.cards) == 0 {
		body := shared.StyleMuted.Render("  No contexts found in talosconfig.")
		return header + "\n\n" + body
	}

	cols, _ := computeGrid(m.width, len(m.cards))

	rendered := make([]string, len(m.cards))
	for i, c := range m.cards {
		rendered[i] = renderCard(c, i == m.cursor)
	}

	// Group into rows of `cols` cards; add gutter between cards in a row.
	var rows []string
	for i := 0; i < len(rendered); i += cols {
		end := i + cols
		if end > len(rendered) {
			end = len(rendered)
		}
		row := joinWithGutter(rendered[i:end])
		rows = append(rows, row)
	}

	// Vertical scrolling so the selected card stays visible.
	availHeight := m.height - headerHeight - 1 // -1 for status bar
	if availHeight < cardHeight {
		availHeight = cardHeight
	}
	rowsPerScreen := (availHeight + rowGutter) / (cardHeight + rowGutter)
	if rowsPerScreen < 1 {
		rowsPerScreen = 1
	}

	selectedRow := m.cursor / cols
	startRow := 0
	if selectedRow >= rowsPerScreen {
		startRow = selectedRow - rowsPerScreen + 1
	}
	endRow := startRow + rowsPerScreen
	if endRow > len(rows) {
		endRow = len(rows)
	}

	visible := rows[startRow:endRow]
	var spaced []string
	for i, r := range visible {
		spaced = append(spaced, r)
		if i < len(visible)-1 {
			spaced = append(spaced, "")
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left, spaced...)

	return header + "\n\n" + body
}

func (m Model) renderHeader() string {
	title := shared.StyleHeader.Render("Multi-Cluster Grid")
	ready, errCount := 0, 0
	for _, c := range m.cards {
		switch c.status {
		case cardReady:
			ready++
		case cardError:
			errCount++
		}
	}
	status := shared.StyleMuted.Render(
		fmt.Sprintf("  %d/%d loaded", ready, len(m.cards)))
	if errCount > 0 {
		status += shared.StyleError.Render(fmt.Sprintf("  %d error(s)", errCount))
	}
	hints := shared.StyleMuted.Render("   " + m.Hints())
	return title + status + hints
}

// joinWithGutter horizontally concatenates cards with a single-column gutter.
func joinWithGutter(cards []string) string {
	if len(cards) == 0 {
		return ""
	}
	gutter := lipgloss.NewStyle().
		Width(colGutter).
		Render(" ")
	parts := make([]string, 0, len(cards)*2)
	for i, c := range cards {
		if i > 0 {
			parts = append(parts, gutter)
		}
		parts = append(parts, c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderCard draws a single 40×5 card for one cluster context.
// The card is composed manually (top border with embedded title, 3 content
// lines with side borders, bottom border) because we want a title band in
// the top border rather than a separate header row.
//
// Selected cards use heavy box-drawing characters and a bright foreground so
// the focused cluster is unmistakable against all-cyan defaults.
func renderCard(c card, selected bool) string {
	var (
		borderColor = shared.ColorSecondary
		tl, tr, bl, br, hz, vt = "╭", "╮", "╰", "╯", "─", "│"
	)
	if selected {
		borderColor = shared.ColorHighlight
		tl, tr, bl, br, hz, vt = "┏", "┓", "┗", "┛", "━", "┃"
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(selected)

	innerW := cardWidth - 2 // space between the two vertical bars

	// Title strip: " ctx-name ─── version "
	right := ""
	if c.summary != nil && c.summary.TalosVersion != "" {
		right = c.summary.TalosVersion
	}
	titleBand := buildTitleBand(c.context, right, innerW, hz)
	top := borderStyle.Render(tl + titleBand + tr)
	bottom := borderStyle.Render(bl + strings.Repeat(hz, innerW) + br)
	vbar := borderStyle.Render(vt)

	// Three content lines.
	bodyLines := renderCardBody(c, innerW)
	framed := make([]string, 0, cardHeight)
	framed = append(framed, top)
	for _, line := range bodyLines {
		framed = append(framed, vbar+padToWidth(line, innerW)+vbar)
	}
	framed = append(framed, bottom)
	return strings.Join(framed, "\n")
}

// padToWidth right-pads a visible-width-aware string to exactly `w` columns.
func padToWidth(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}

// buildTitleBand formats a title strip sized to exactly `innerW` columns
// visible width, using `hz` as filler between left (context name) and right
// (version) labels. The fill character matches the surrounding border so the
// band blends with the top edge.
func buildTitleBand(left, right string, innerW int, hz string) string {
	leftLabel := " " + shared.Truncate(left, innerW-4) + " "
	rightLabel := ""
	if right != "" {
		rightLabel = " " + shared.Truncate(right, innerW/3) + " "
	}
	fillLen := innerW - lipgloss.Width(leftLabel) - lipgloss.Width(rightLabel)
	if fillLen < 0 {
		fillLen = 0
	}
	fill := strings.Repeat(hz, fillLen)
	return leftLabel + fill + rightLabel
}

// renderCardBody returns exactly 3 content lines for the card body.
func renderCardBody(c card, innerW int) []string {
	lines := make([]string, 0, 3)
	switch c.status {
	case cardLoading:
		lines = append(lines, "  "+shared.StyleMuted.Render("⋯ Loading..."))
		lines = append(lines, "")
		lines = append(lines, "")
	case cardError:
		lines = append(lines, " "+shared.StyleError.Render("● Unreachable"))
		msg := ""
		if c.err != nil {
			msg = shared.Truncate(c.err.Error(), innerW-2)
		}
		lines = append(lines, " "+shared.StyleMuted.Render(msg))
		lines = append(lines, "")
	case cardReady:
		s := c.summary
		countStr := fmt.Sprintf(" %d CP  %d W   ", s.CPCount, s.WorkerCount)
		dotArea := innerW - lipgloss.Width(countStr)
		if dotArea < 4 {
			dotArea = 4
		}
		dots := renderDotRow(s, dotArea)
		lines = append(lines, countStr+dots)
		lines = append(lines, " "+healthLine(s))
		ago := "—"
		if !s.FetchedAt.IsZero() {
			ago = time.Since(s.FetchedAt).Truncate(time.Second).String() + " ago"
		}
		lines = append(lines, shared.StyleMuted.Render(" updated "+ago))
	}
	for len(lines) < 3 {
		lines = append(lines, "")
	}
	return lines[:3]
}

// renderDotRow produces a wrap-free row of colored dots representing node
// health, trimmed to `maxWidth` visible columns. Uses shared.NodeDotStyle so
// the grid's dots match the dashboard cluster-nodes panel exactly (error /
// warn / ready / offline).
func renderDotRow(s *clusterSummary, maxWidth int) string {
	if s == nil || len(s.Nodes) == 0 {
		return shared.StyleMuted.Render("no nodes")
	}
	// Each "● " is 2 columns, except the last has no trailing space.
	maxDots := (maxWidth + 1) / 2
	if maxDots < 1 {
		maxDots = 1
	}
	if maxDots > len(s.Nodes) {
		maxDots = len(s.Nodes)
	}

	hasAnyData := len(s.ServicesByNode) > 0
	var dots []string
	for i := 0; i < maxDots; i++ {
		n := s.Nodes[i]
		svcs, hasSvcs := s.ServicesByNode[n.Hostname]
		failed := false
		for _, sv := range svcs {
			if sv.Health == shared.HealthFailed {
				failed = true
				break
			}
		}
		memPct := shared.MemUsedPct(s.MemoryByNode[n.Hostname])
		cpuPct := s.CPUByNode[n.Hostname].UsagePercent
		icon, style := shared.NodeDotStyle(hasSvcs, hasAnyData, failed, memPct, cpuPct)
		dots = append(dots, style.Render(icon))
	}
	return strings.Join(dots, " ")
}

// healthLine returns the "Healthy / N failed / N unreachable" summary.
func healthLine(s *clusterSummary) string {
	switch {
	case s.FailedCount > 0 && s.UnreachableCount > 0:
		return shared.StyleError.Render(
			fmt.Sprintf("%s %d failed · %d unreachable",
				shared.StatusIcon("Failed"), s.FailedCount, s.UnreachableCount))
	case s.FailedCount > 0:
		return shared.StyleError.Render(
			fmt.Sprintf("%s %d failed", shared.StatusIcon("Failed"), s.FailedCount))
	case s.UnreachableCount > 0:
		return shared.StyleWarning.Render(
			fmt.Sprintf("%s %d unreachable",
				shared.StatusIcon("Stopped"), s.UnreachableCount))
	default:
		return shared.StyleSuccess.Render(
			shared.StatusIcon("Running") + " Healthy")
	}
}
