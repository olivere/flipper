package layout

import "fmt"

// Item represents a single row in an item list.
type Item struct {
	Index       int // displayed if > 0
	Title       string
	Description string // optional secondary line
}

// DrawItems draws a vertical list of items within r, separated by
// dotted dividers. Returns the number of items that fit within r.
func (c *Canvas) DrawItems(r Rect, items []Item) int {
	s := c.Scale()
	padX := 12.0 * s
	padY := 8.0 * s
	indexW := 28.0 * s // space reserved for index number

	titlePts := FontSize(TextBody, s)
	descPts := FontSize(TextCaption, s)
	rowH := titlePts + descPts + padY*2.5

	y := float64(r.Y) + padY
	drawn := 0

	for i, item := range items {
		if y+rowH > float64(r.Y+r.H) {
			break
		}

		x := float64(r.X) + padX
		titleY := y + titlePts

		// Index number
		if item.Index > 0 {
			c.DrawText(fmt.Sprintf("%d.", item.Index), x, titleY, FontBold, TextBody)
		}

		// Title
		titleX := x
		if item.Index > 0 {
			titleX += indexW
		}
		c.DrawText(item.Title, titleX, titleY, FontBold, TextBody)

		// Description
		if item.Description != "" {
			descY := titleY + descPts + 4*s
			c.DrawText(item.Description, titleX, descY, FontRegular, TextCaption)
		}

		y += rowH
		drawn++

		// Dotted divider between items (not after last)
		if i < len(items)-1 {
			c.HLineDotted(Rect{
				X: r.X + int(padX),
				Y: 0,
				W: r.W - 2*int(padX),
				H: 1,
			}, int(y)-1)
			y += 2 * s
		}
	}

	return drawn
}
