package layout

// Rect is an axis-aligned rectangle in pixel coordinates.
type Rect struct {
	X, Y, W, H int
}

// Inset returns a new Rect shrunk by the given padding.
func (r Rect) Inset(top, right, bottom, left int) Rect {
	return Rect{
		X: r.X + left,
		Y: r.Y + top,
		W: r.W - left - right,
		H: r.H - top - bottom,
	}
}

// InsetAll returns a new Rect shrunk by uniform padding on all sides.
func (r Rect) InsetAll(p int) Rect {
	return r.Inset(p, p, p, p)
}

// SplitH splits the rect horizontally at height h from the top,
// returning the top and bottom portions.
func (r Rect) SplitH(h int) (top, bottom Rect) {
	top = Rect{X: r.X, Y: r.Y, W: r.W, H: h}
	bottom = Rect{X: r.X, Y: r.Y + h, W: r.W, H: r.H - h}
	return
}

// SplitV splits the rect vertically at width w from the left,
// returning the left and right portions.
func (r Rect) SplitV(w int) (left, right Rect) {
	left = Rect{X: r.X, Y: r.Y, W: w, H: r.H}
	right = Rect{X: r.X + w, Y: r.Y, W: r.W - w, H: r.H}
	return
}

// Columns divides the rect into n equal-width columns with the given gap.
func (r Rect) Columns(n, gap int) []Rect {
	if n <= 0 {
		return nil
	}
	totalGap := gap * (n - 1)
	colW := (r.W - totalGap) / n
	cols := make([]Rect, n)
	x := r.X
	for i := range n {
		cols[i] = Rect{X: x, Y: r.Y, W: colW, H: r.H}
		x += colW + gap
	}
	return cols
}

// Rows divides the rect into n equal-height rows with the given gap.
func (r Rect) Rows(n, gap int) []Rect {
	if n <= 0 {
		return nil
	}
	totalGap := gap * (n - 1)
	rowH := (r.H - totalGap) / n
	rows := make([]Rect, n)
	y := r.Y
	for i := range n {
		rows[i] = Rect{X: r.X, Y: y, W: r.W, H: rowH}
		y += rowH + gap
	}
	return rows
}
