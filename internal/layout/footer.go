package layout

import (
	"image/color"
)

// Footer reserves space for a footer bar at the bottom of bounds.
// Returns the content area (above the footer) and the footer Rect.
// The height is in logical pixels (scaled internally).
func (c *Canvas) Footer(bounds Rect, height int) (content, footer Rect) {
	h := int(float64(height) * c.Scale())
	content = Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: bounds.H - h}
	footer = Rect{X: bounds.X, Y: bounds.Y + bounds.H - h, W: bounds.W, H: h}
	return
}

// DrawFooter renders the standard TRMNL footer: light gray background
// with title on the left and optional right-aligned text.
func (c *Canvas) DrawFooter(r Rect, title, right string) {
	s := c.Scale()

	// Light gray background
	c.dc.SetColor(color.NRGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff})
	c.dc.DrawRectangle(float64(r.X), float64(r.Y), float64(r.W), float64(r.H))
	c.dc.Fill()

	// Title on the left
	c.dc.SetColor(color.Black)
	pad := 16.0 * s
	cy := float64(r.Y) + float64(r.H)/2.0
	c.DrawText(title, float64(r.X)+pad, cy, FontBold, TextCaption)

	// Right-aligned text
	if right != "" {
		c.DrawTextRight(right, float64(r.X+r.W)-pad, cy, FontRegular, TextCaption)
	}
}
