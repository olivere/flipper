package demo

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/layout"
	"github.com/olivere/flipper/internal/screen"
)

// templateFunc is a function that creates a Template from bounds.
type templateFunc struct {
	name string
	fn   func(layout.Rect) layout.Template
}

var templates = []templateFunc{
	{"Full", layout.Full},
	{"HalfVertical", layout.HalfVertical},
	{"HalfHorizontal", layout.HalfHorizontal},
	{"Rows3", layout.Rows3},
	{"Quadrant", layout.Quadrant},
	{"LeftStack", layout.LeftStack},
	{"RightStack", layout.RightStack},
	{"TopSplit", layout.TopSplit},
	{"BottomSplit", layout.BottomSplit},
	{"Grid2x3", layout.Grid2x3},
}

// sampleItems are used in the item list demo variant.
var sampleItems = []layout.Item{
	{Index: 1, Title: "Display error when polling", Description: "due 10 Feb"},
	{Index: 2, Title: "Native integration with HA", Description: "due 12 Feb"},
	{Index: 3, Title: "Native integration with Basecamp", Description: "due 15 Feb"},
	{Index: 4, Title: "Native integration with Ticktick", Description: "due 11 Feb"},
	{Index: 5, Title: "Increase server throughput", Description: "due 10 Feb"},
	{Index: 6, Title: "Improve DevX (developer experience)", Description: "due 26 Feb"},
}

func init() {
	screen.Register("demo", func(_ *config.Config, _ map[string]any) (screen.Screen, error) {
		return New(), nil
	})
}

// Screen cycles through layout templates to visualize them.
type Screen struct {
	mu    sync.Mutex
	index int
}

func New() *Screen {
	return &Screen{}
}

func (s *Screen) Name() string { return "demo" }

func (s *Screen) Render(_ context.Context, opts screen.RenderOpts) (image.Image, error) {
	s.mu.Lock()
	// Cycle through templates, then show an item list demo
	total := len(templates) + 1 // +1 for item list
	idx := s.index % total
	s.index = (s.index + 1) % total
	s.mu.Unlock()

	c := layout.New(opts.Width, opts.Height, opts.ColorMode)

	if idx == len(templates) {
		// Item list demo
		return renderItemDemo(c), nil
	}

	tf := templates[idx]

	// Reserve footer
	content, footer := c.Footer(c.Bounds(), 40)
	c.DrawFooter(footer, "Layout Demo", tf.name)

	// Apply template to content area with padding
	padded := content.InsetAll(int(8 * c.Scale()))
	tmpl := tf.fn(padded)

	// Draw each region
	colors := regionColors(opts.ColorMode)
	for i, region := range tmpl.Regions {
		drawRegion(c, region, colors[i%len(colors)])
	}

	return c.Image(), nil
}

func renderItemDemo(c *layout.Canvas) image.Image {
	content, footer := c.Footer(c.Bounds(), 40)
	c.DrawFooter(footer, "Layout Demo", "Item List")

	body := content.InsetAll(int(8 * c.Scale()))
	c.DrawItems(body, sampleItems)

	return c.Image()
}

// drawRegion draws a labeled region with a border and background.
func drawRegion(c *layout.Canvas, region layout.Region, bg color.Color) {
	r := region.Rect
	s := c.Scale()

	// Fill background
	c.DC().SetColor(bg)
	c.DC().DrawRectangle(float64(r.X), float64(r.Y), float64(r.W), float64(r.H))
	c.DC().Fill()

	// Border
	c.DC().SetColor(color.Black)
	lineW := 2.0 * s
	c.DC().SetLineWidth(lineW)
	c.DC().DrawRectangle(float64(r.X), float64(r.Y), float64(r.W), float64(r.H))
	c.DC().Stroke()

	// Region name centered
	cx := float64(r.X) + float64(r.W)/2
	cy := float64(r.Y) + float64(r.H)/2
	c.DrawTextCenter(region.Name, cx, cy-10*s, layout.FontBold, layout.TextHeading)

	// Dimensions below the name
	dims := fmt.Sprintf("%d x %d", r.W, r.H)
	c.DrawTextCenter(dims, cx, cy+14*s, layout.FontRegular, layout.TextCaption)
}

// regionColors returns background colors appropriate for the device.
func regionColors(colorMode display.ColorMode) []color.Color {
	if colorMode == display.ColorGray4 {
		return []color.Color{
			color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // white
			color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}, // light gray
			color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff},
			color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff},
		}
	}
	// BW: all white backgrounds (borders provide contrast)
	return []color.Color{
		color.White,
		color.White,
		color.White,
		color.White,
		color.White,
		color.White,
	}
}
