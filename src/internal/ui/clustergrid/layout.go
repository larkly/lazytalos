package clustergrid

const (
	cardWidth    = 40
	cardHeight   = 5 // top border + 3 content + bottom border
	colGutter    = 1
	rowGutter    = 1
	headerHeight = 2 // title line + blank separator
)

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
