package news

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"sync"
	"time"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/layout"
	"github.com/olivere/flipper/internal/screen"
)

func init() {
	screen.Register("news", func(_ *config.Config, params map[string]any) (screen.Screen, error) {
		refreshStr := screen.ParamString(params, "refresh_interval", "30m")
		refreshInterval, err := time.ParseDuration(refreshStr)
		if err != nil {
			refreshInterval = 30 * time.Minute
		}

		return New(Config{
			Search:          screen.ParamString(params, "search", ""),
			Title:           screen.ParamString(params, "title", ""),
			Lang:            screen.ParamString(params, "lang", "en"),
			Country:         screen.ParamString(params, "country", "US"),
			Limit:           screen.ParamInt(params, "limit", 10),
			RefreshInterval: refreshInterval,
		}), nil
	})
}

// Config holds news screen parameters.
type Config struct {
	Search          string
	Title           string // overrides footer text (defaults to Search or "Top Headlines")
	Lang            string
	Country         string
	Limit           int
	RefreshInterval time.Duration
}

// Screen renders latest news headlines from Google News RSS.
type Screen struct {
	cfg    Config
	mu     sync.Mutex
	cache  []Item
	expiry time.Time
}

func New(cfg Config) *Screen {
	if cfg.Lang == "" {
		cfg.Lang = "en"
	}
	if cfg.Country == "" {
		cfg.Country = "US"
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 10
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 30 * time.Minute
	}
	return &Screen{cfg: cfg}
}

// SetCache pre-fills the screen's item cache with the given expiry.
// Intended for previews and testing.
func (s *Screen) SetCache(items []Item, expiry time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = items
	s.expiry = expiry
}

func (s *Screen) Name() string { return "news" }

func (s *Screen) Render(ctx context.Context, opts screen.RenderOpts) (image.Image, error) {
	items, err := s.getData(ctx)
	if err != nil {
		return s.renderError(opts, err), nil
	}
	return s.renderNews(opts, items), nil
}

// getData returns cached news items or fetches fresh data.
func (s *Screen) getData(ctx context.Context) ([]Item, error) {
	s.mu.Lock()
	if s.cache != nil && time.Now().Before(s.expiry) {
		items := s.cache
		s.mu.Unlock()
		return items, nil
	}
	cached := s.cache
	s.mu.Unlock()

	// Fetch outside the lock so concurrent renders aren't blocked.
	items, err := fetch(ctx, s.cfg)
	if err != nil {
		if cached != nil {
			slog.Warn("news fetch failed, using cache", "search", s.cfg.Search, "error", err)
			return cached, nil
		}
		return nil, fmt.Errorf("fetch news: %w", err)
	}

	s.mu.Lock()
	s.cache = items
	s.expiry = time.Now().Add(s.cfg.RefreshInterval)
	s.mu.Unlock()
	return items, nil
}

// renderNews draws the news headlines list.
func (s *Screen) renderNews(opts screen.RenderOpts, items []Item) image.Image {
	c := layout.New(opts.Width, opts.Height, opts.ColorMode)

	footerRight := "Top Headlines"
	if s.cfg.Title != "" {
		footerRight = s.cfg.Title
	} else if s.cfg.Search != "" {
		footerRight = s.cfg.Search
	}

	content, footer := c.Footer(c.Bounds(), 40)
	c.DrawFooter(footer, "News", footerRight)

	layoutItems := make([]layout.Item, len(items))
	for i, item := range items {
		desc := item.Source
		if rt := relativeTime(item.PubDate); rt != "" {
			if desc != "" {
				desc += " · "
			}
			desc += rt
		}
		layoutItems[i] = layout.Item{
			Index:       i + 1,
			Title:       item.Title,
			Description: desc,
		}
	}

	c.DrawItems(content, layoutItems)
	return c.Image()
}

// renderError draws a fallback screen when news data is unavailable.
func (s *Screen) renderError(opts screen.RenderOpts, err error) image.Image {
	c := layout.New(opts.Width, opts.Height, opts.ColorMode)

	content, footer := c.Footer(c.Bounds(), 40)
	c.DrawFooter(footer, "News", "Error")

	cx := float64(content.X) + float64(content.W)/2
	cy := float64(content.Y) + float64(content.H)/2

	c.DrawTextCenter("News Unavailable", cx, cy-20*c.Scale(), layout.FontBold, layout.TextTitle)
	c.DrawTextCenter(err.Error(), cx, cy+20*c.Scale(), layout.FontRegular, layout.TextCaption)

	return c.Image()
}
