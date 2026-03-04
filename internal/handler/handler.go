package handler

import (
	"log/slog"
	"sync"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/screen"
)

type Handler struct {
	Config   *config.Config
	Devices  *device.Registry
	Screens  *screen.Registry
	Pipeline *display.Pipeline
	Cache    *ImageCache
	Logger   *slog.Logger
}

// ImageCache stores processed images in memory.
type ImageCache struct {
	mu         sync.RWMutex
	byKey      map[string]*display.Result
	byFilename map[string]*display.Result
}

func NewImageCache() *ImageCache {
	return &ImageCache{
		byKey:      make(map[string]*display.Result),
		byFilename: make(map[string]*display.Result),
	}
}

func (c *ImageCache) Get(key string) *display.Result {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byKey[key]
}

func (c *ImageCache) GetByFilename(filename string) *display.Result {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byFilename[filename]
}

func (c *ImageCache) Set(key string, result *display.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byKey[key] = result
	c.byFilename[result.Filename] = result
}
