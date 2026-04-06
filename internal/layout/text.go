package layout

// TextSize defines preset text sizes that scale with device resolution.
type TextSize int

const (
	TextHero    TextSize = iota // Very large numbers (64pt base)
	TextTitle                   // Large headers (32pt base)
	TextHeading                 // Section headers (24pt base)
	TextBody                    // Normal text (18pt base)
	TextCaption                 // Small labels (14pt base)
	TextSmall                   // Tiny text (11pt base)
)

// baseFontSize returns the base point size for a TextSize at scale 1.0.
func baseFontSize(ts TextSize) float64 {
	switch ts {
	case TextHero:
		return 64
	case TextTitle:
		return 32
	case TextHeading:
		return 24
	case TextBody:
		return 18
	case TextCaption:
		return 14
	case TextSmall:
		return 11
	default:
		return 18
	}
}

// FontSize returns the point size for a TextSize at the given scale.
func FontSize(ts TextSize, scale float64) float64 {
	return baseFontSize(ts) * scale
}

// setFont configures the canvas font face for the given style and size.
// Returns false if the font could not be loaded.
func (c *Canvas) setFont(style FontStyle, size TextSize) bool {
	pts := FontSize(size, c.Scale())
	face, err := Face(style, pts)
	if err != nil {
		return false
	}
	c.dc.SetFontFace(face)
	return true
}

// DrawText draws a single line of text at position (x, y).
// Text is left-aligned and vertically centered at y.
func (c *Canvas) DrawText(text string, x, y float64, style FontStyle, size TextSize) {
	if !c.setFont(style, size) {
		return
	}
	c.dc.DrawStringAnchored(text, x, y, 0, 0.5)
}

// DrawTextRight draws a single line of text right-aligned at (x, y).
// x is the right edge; text is drawn to the left of it.
func (c *Canvas) DrawTextRight(text string, x, y float64, style FontStyle, size TextSize) {
	if !c.setFont(style, size) {
		return
	}
	c.dc.DrawStringAnchored(text, x, y, 1, 0.5)
}

// DrawTextCenter draws a single line of text centered at (x, y).
func (c *Canvas) DrawTextCenter(text string, x, y float64, style FontStyle, size TextSize) {
	if !c.setFont(style, size) {
		return
	}
	c.dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
}

// DrawTextWrap draws word-wrapped text within the given Rect.
// Returns the Y position after the last line drawn.
func (c *Canvas) DrawTextWrap(text string, r Rect, style FontStyle, size TextSize) float64 {
	if !c.setFont(style, size) {
		return float64(r.Y)
	}

	pts := FontSize(size, c.Scale())
	lineHeight := pts * 1.4
	lines := c.dc.WordWrap(text, float64(r.W))

	y := float64(r.Y) + pts
	for _, line := range lines {
		if y > float64(r.Y+r.H) {
			break
		}
		c.dc.DrawString(line, float64(r.X), y)
		y += lineHeight
	}
	return y
}

// MeasureText returns the width and height of a single line of text.
func (c *Canvas) MeasureText(text string, style FontStyle, size TextSize) (w, h float64) {
	if !c.setFont(style, size) {
		return 0, 0
	}
	return c.dc.MeasureString(text)
}
