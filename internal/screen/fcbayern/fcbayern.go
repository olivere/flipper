package fcbayern

import (
	"context"
	"image"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/layout"
	"github.com/olivere/flipper/internal/screen"
)

func init() {
	screen.Register("fcbayern", func(_ *config.Config, _ map[string]any) (screen.Screen, error) {
		return New(Config{}), nil
	})
}

// Config holds FC Bayern screen parameters.
type Config struct{}

// Screen renders upcoming FC Bayern München matches.
type Screen struct {
	cfg Config
}

func New(cfg Config) *Screen {
	return &Screen{cfg: cfg}
}

func (s *Screen) Name() string { return "fcbayern" }

func (s *Screen) Render(_ context.Context, opts screen.RenderOpts) (image.Image, error) {
	c := layout.New(opts.Width, opts.Height, opts.ColorMode)

	content, footer := c.Footer(c.Bounds(), 40)
	c.DrawFooter(footer, "FC Bayern", "Next Games")

	cx := float64(content.X) + float64(content.W)/2
	cy := float64(content.Y) + float64(content.H)/2

	c.DrawTextCenter("FC Bayern München", cx, cy-20*c.Scale(), layout.FontBold, layout.TextTitle)
	c.DrawTextCenter("Next Games", cx, cy+20*c.Scale(), layout.FontRegular, layout.TextBody)

	return c.Image(), nil
}
