package layout

import (
	"image/color"
	"math"
)

// DrawBadge draws an inverted pill/badge (white text on black background)
// at position (x, y). Returns the total width consumed including padding,
// for inline layout of multiple badges.
func (c *Canvas) DrawBadge(text string, x, y float64, size TextSize) float64 {
	s := c.Scale()
	padX := 6.0 * s
	padY := 3.0 * s
	radius := 3.0 * s

	w, h := c.MeasureText(text, FontBold, size)

	// Black rounded rect background
	c.dc.SetColor(color.Black)
	c.dc.DrawRoundedRectangle(x, y-h/2-padY, w+2*padX, h+2*padY, radius)
	c.dc.Fill()

	// White text inside
	c.dc.SetColor(color.White)
	c.setFont(FontBold, size)
	c.dc.DrawStringAnchored(text, x+padX, y, 0, 0.5)

	// Reset to black
	c.dc.SetColor(color.Black)

	return w + 2*padX + math.Max(4.0, 4.0*s)
}
