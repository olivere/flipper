package handler

import (
	"log/slog"
	"sync"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/screen"
)

// Handler holds the shared dependencies for all HTTP handlers.
type Handler struct {
	Config   *config.Config
	Devices  *device.Registry
	Screens  *screen.Registry
	Pipeline *display.Pipeline
	Cache    *ImageCache
	Logger   *slog.Logger
}

// ImageCache stores processed images by filename so the image endpoint
// can serve them after /api/display hands out the URL.
type ImageCache struct {
	mu         sync.RWMutex
	byFilename map[string]*display.Result
}

func NewImageCache() *ImageCache {
	return &ImageCache{
		byFilename: make(map[string]*display.Result),
	}
}

func (c *ImageCache) GetByFilename(filename string) *display.Result {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byFilename[filename]
}

// maxCacheEntries limits the number of processed images kept in memory.
// When exceeded, the oldest entries are evicted by clearing the map.
const maxCacheEntries = 64

func (c *ImageCache) Set(result *display.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.byFilename) >= maxCacheEntries {
		clear(c.byFilename)
	}
	c.byFilename[result.Filename] = result
}
