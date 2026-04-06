package layout

import (
	"image"
	"image/color"

	"github.com/fogleman/gg"

	"github.com/olivere/flipper/internal/display"
)

// Canvas wraps a gg.Context for e-ink rendering. It knows the device
// dimensions and color capabilities.
type Canvas struct {
	dc        *gg.Context
	width     int
	height    int
	colorMode display.ColorMode
}

// New creates a Canvas for the given dimensions with a white background.
func New(width, height int, colorMode display.ColorMode) *Canvas {
	dc := gg.NewContext(width, height)
	dc.SetColor(color.White)
	dc.Clear()
	dc.SetColor(color.Black)
	return &Canvas{dc: dc, width: width, height: height, colorMode: colorMode}
}

// Image returns the rendered image.
func (c *Canvas) Image() image.Image {
	return c.dc.Image()
}

// Scale returns the scaling factor relative to the OG device (800px).
// Authors work in 800px logical coordinates; all pixel values should
// be multiplied by Scale().
func (c *Canvas) Scale() float64 {
	return float64(c.width) / 800.0
}

// ColorMode returns the device's color capability.
func (c *Canvas) ColorMode() display.ColorMode {
	return c.colorMode
}

// Bounds returns the full canvas as a Rect.
func (c *Canvas) Bounds() Rect {
	return Rect{X: 0, Y: 0, W: c.width, H: c.height}
}

// DC exposes the underlying gg.Context for custom drawing.
func (c *Canvas) DC() *gg.Context {
	return c.dc
}

// Sub creates a new Canvas for an isolated sub-region. Drawing on the
// sub-canvas does not affect the parent. Use Draw to composite it back.
func (c *Canvas) Sub(r Rect) *Canvas {
	return New(r.W, r.H, c.colorMode)
}

// Draw composites a sub-canvas onto this canvas at position (x, y).
func (c *Canvas) Draw(sub *Canvas, x, y int) {
	c.dc.DrawImage(sub.Image(), x, y)
}
