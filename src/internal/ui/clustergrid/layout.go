package clustergrid

const (
	cardWidth    = 40
	cardHeight   = 5 // top border + 3 content + bottom border
	colGutter    = 1
	rowGutter    = 1
	headerHeight = 2 // title line + blank separator
)

// columns returns the number of columns that fit in the current width.
func (m Model) columns() int {
	if m.width <= 0 {
		return 1
	}
	cols := (m.width + colGutter) / (cardWidth + colGutter)
	if cols < 1 {
		return 1
	}
	return cols
}

// computeGrid returns (cols, rows) for a given width and card count.
// Exposed for tests.
func computeGrid(width, count int) (cols, rows int) {
	if count == 0 {
		return 1, 0
	}
	if width <= 0 {
		width = cardWidth
	}
	cols = (width + colGutter) / (cardWidth + colGutter)
	if cols < 1 {
		cols = 1
	}
	rows = (count + cols - 1) / cols
	return cols, rows
}
