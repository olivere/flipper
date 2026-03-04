package screen

import (
	"context"
	"image"
	"sync"
)

// RenderOpts describes the target device dimensions and scaling mode.
type RenderOpts struct {
	Width   int
	Height  int
	Scaling string // "fit" or "fill"
}

// Screen produces images for display on a TRMNL device.
type Screen interface {
	Name() string
	Render(ctx context.Context, opts RenderOpts) (image.Image, error)
}

// Registry holds screens and supports round-robin rotation.
type Registry struct {
	mu      sync.Mutex
	screens []Screen
	index   int
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Add(s Screen) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.screens = append(r.screens, s)
}

// Current returns the current screen without advancing.
func (r *Registry) Current() Screen {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.screens) == 0 {
		return nil
	}
	return r.screens[r.index%len(r.screens)]
}

// Next advances to the next screen and returns it.
func (r *Registry) Next() Screen {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.screens) == 0 {
		return nil
	}
	r.index = (r.index + 1) % len(r.screens)
	return r.screens[r.index]
}

func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.screens)
}
