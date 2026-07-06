package screen

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"sync"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/display"
)

// Factory creates a Screen from the application config and optional
// playlist params. Each screen package registers a factory via init().
type Factory func(cfg *config.Config, params map[string]any) (Screen, error)

var (
	factoryMu sync.RWMutex
	factories = map[string]Factory{}
)

// Register adds a screen factory under the given name. Typically called
// from a screen package's init() function.
func Register(name string, f Factory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[name] = f
}

// Build creates a Screen by name using the registered factory.
// Returns an error if the name is unknown.
func Build(name string, cfg *config.Config, params map[string]any) (Screen, error) {
	factoryMu.RLock()
	f, ok := factories[name]
	factoryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown screen type: %q", name)
	}
	return f(cfg, params)
}

// ParamString extracts a string from a params map with a fallback.
func ParamString(params map[string]any, key, fallback string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return fallback
}

// ParamInt extracts an int from a params map with a fallback.
// Handles int64 (TOML integers), float64, and string values.
func ParamInt(params map[string]any, key string, fallback int) int {
	v, ok := params[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return fallback
}

// ParamFloat extracts a float64 from a params map with a fallback.
// Handles float64, int64 (TOML integers), and string values.
func ParamFloat(params map[string]any, key string, fallback float64) float64 {
	v, ok := params[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f
		}
	}
	return fallback
}

// RenderOpts describes the target device dimensions and scaling mode.
type RenderOpts struct {
	Width     int
	Height    int
	Scaling   string            // "fit" or "fill"
	ColorMode display.ColorMode // device color capability
}

// Screen produces images for display on a TRMNL device.
type Screen interface {
	Name() string
	Render(ctx context.Context, opts RenderOpts) (image.Image, error)
}

// Registry holds screens and supports round-robin rotation. Each
// device rotates independently, keyed by its ID, so multiple devices
// polling the same server each see the full sequence.
type Registry struct {
	mu      sync.Mutex
	screens []Screen
	pos     cursors
}

// NewRegistry returns an empty screen registry.
func NewRegistry() *Registry {
	return &Registry{pos: cursors{}}
}

func (r *Registry) Add(s Screen) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.screens = append(r.screens, s)
}

// Current returns deviceID's current screen without advancing.
func (r *Registry) Current(deviceID string) Screen {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.screens) == 0 {
		return nil
	}
	return r.screens[r.pos.current(deviceID, len(r.screens))]
}

// Next returns deviceID's current screen and advances that device's
// cursor to the next one.
func (r *Registry) Next(deviceID string) Screen {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.screens) == 0 {
		return nil
	}
	return r.screens[r.pos.next(deviceID, len(r.screens))]
}

func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.screens)
}

// All returns a copy of the registered screens.
func (r *Registry) All() []Screen {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Screen, len(r.screens))
	copy(out, r.screens)
	return out
}
