package screen

import (
	"context"
	"image"
	"testing"
	"time"
)

type fakeScreen struct {
	name string
}

func (s *fakeScreen) Name() string { return s.name }

func (s *fakeScreen) Render(ctx context.Context, opts RenderOpts) (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

func TestPlaylistPerDeviceCursor(t *testing.T) {
	p := NewPlaylist()
	p.Add(&fakeScreen{name: "a"}, time.Minute)
	p.Add(&fakeScreen{name: "b"}, time.Minute)
	p.Add(&fakeScreen{name: "c"}, time.Minute)

	// Two devices polling alternately must each see the full sequence.
	for _, want := range []string{"a", "b", "c", "a"} {
		s1, _ := p.Next("dev1")
		s2, _ := p.Next("dev2")
		if s1.Name() != want {
			t.Errorf("dev1: got %s, want %s", s1.Name(), want)
		}
		if s2.Name() != want {
			t.Errorf("dev2: got %s, want %s", s2.Name(), want)
		}
	}
}

func TestPlaylistEmpty(t *testing.T) {
	p := NewPlaylist()
	if s, d := p.Next("dev1"); s != nil || d != 0 {
		t.Errorf("expected (nil, 0) from empty playlist, got (%v, %v)", s, d)
	}
}

func TestRegistryPerDeviceCursor(t *testing.T) {
	r := NewRegistry()
	r.Add(&fakeScreen{name: "a"})
	r.Add(&fakeScreen{name: "b"})

	for _, want := range []string{"a", "b", "a"} {
		if got := r.Next("dev1").Name(); got != want {
			t.Errorf("dev1: got %s, want %s", got, want)
		}
		if got := r.Next("dev2").Name(); got != want {
			t.Errorf("dev2: got %s, want %s", got, want)
		}
	}
}

func TestRegistryCurrentDoesNotAdvance(t *testing.T) {
	r := NewRegistry()
	r.Add(&fakeScreen{name: "a"})
	r.Add(&fakeScreen{name: "b"})

	if got := r.Current("dev1").Name(); got != "a" {
		t.Errorf("got %s, want a", got)
	}
	if got := r.Current("dev1").Name(); got != "a" {
		t.Errorf("Current advanced the cursor: got %s, want a", got)
	}

	r.Next("dev1")
	if got := r.Current("dev1").Name(); got != "b" {
		t.Errorf("got %s, want b", got)
	}
	// Another device's cursor is unaffected.
	if got := r.Current("dev2").Name(); got != "a" {
		t.Errorf("dev2: got %s, want a", got)
	}
}

func TestRegistryEmpty(t *testing.T) {
	r := NewRegistry()
	if r.Current("dev1") != nil || r.Next("dev1") != nil {
		t.Error("expected nil from empty registry")
	}
}
