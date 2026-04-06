package layout

import (
	"image/color"
	"math"
)

// HLine draws a solid horizontal line across the full width of r
// at vertical offset y within r.
func (c *Canvas) HLine(r Rect, y int) {
	s := c.Scale()
	thick := math.Max(1.0, 2.0*s)
	c.dc.SetColor(color.Black)
	c.dc.SetLineWidth(thick)
	fy := float64(r.Y + y)
	c.dc.DrawLine(float64(r.X), fy, float64(r.X+r.W), fy)
	c.dc.Stroke()
}

// HLineDotted draws a dotted horizontal line across the full width of r
// at vertical offset y within r. Matches the TRMNL dotted separator style.
func (c *Canvas) HLineDotted(r Rect, y int) {
	s := c.Scale()
	dotSize := math.Max(1.0, 2.0*s)
	gap := dotSize * 3
	c.dc.SetColor(color.Black)

	fy := float64(r.Y + y)
	x := float64(r.X)
	endX := float64(r.X + r.W)
	for x < endX {
		c.dc.DrawPoint(x, fy, dotSize/2)
		c.dc.Fill()
		x += gap
	}
}

// VLine draws a solid vertical line across the full height of r
// at horizontal offset x within r.
func (c *Canvas) VLine(r Rect, x int) {
	s := c.Scale()
	thick := math.Max(1.0, 2.0*s)
	c.dc.SetColor(color.Black)
	c.dc.SetLineWidth(thick)
	fx := float64(r.X + x)
	c.dc.DrawLine(fx, float64(r.Y), fx, float64(r.Y+r.H))
	c.dc.Stroke()
}

// VLineDotted draws a dotted vertical line across the full height of r
// at horizontal offset x within r.
func (c *Canvas) VLineDotted(r Rect, x int) {
	s := c.Scale()
	dotSize := math.Max(1.0, 2.0*s)
	gap := dotSize * 3
	c.dc.SetColor(color.Black)

	fx := float64(r.X + x)
	y := float64(r.Y)
	endY := float64(r.Y + r.H)
	for y < endY {
		c.dc.DrawPoint(fx, y, dotSize/2)
		c.dc.Fill()
		y += gap
	}
}
