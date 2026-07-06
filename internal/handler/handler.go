package handler

import (
	"log/slog"
	"sync"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/firmware"
	"github.com/olivere/flipper/internal/screen"
)

// Handler holds the shared dependencies for all HTTP handlers.
type Handler struct {
	Config    *config.Config
	Devices   *device.Registry
	Screens   *screen.Registry
	Playlists *screen.Playlists // nil when no playlists are configured
	Pipeline  *display.Pipeline
	Cache     *ImageCache
	Firmware  *Firmware // nil when firmware support is disabled
	Logger    *slog.Logger
}

// Firmware bundles the local binary store and pending-arm tracker.
// Both are nil-safe to leave unset when firmware support is off; the
// Handler keeps them as a pair because the firmware-dispatch path
// only makes sense when both are present.
type Firmware struct {
	Store   *firmware.Store
	Pending *firmware.Pending
}

// ImageCache stores processed images by filename so the image endpoint
// can serve them after /api/display hands out the URL.
type ImageCache struct {
	mu         sync.RWMutex
	byFilename map[string]*display.Result
	order      []string // insertion order, oldest first
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
// When full, the oldest entry is evicted — never the one just added,
// so an image_url handed to a device stays fetchable.
const maxCacheEntries = 64

func (c *ImageCache) Set(result *display.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.byFilename[result.Filename]; ok {
		return
	}
	for len(c.byFilename) >= maxCacheEntries {
		delete(c.byFilename, c.order[0])
		c.order = c.order[1:]
	}
	c.byFilename[result.Filename] = result
	c.order = append(c.order, result.Filename)
}
