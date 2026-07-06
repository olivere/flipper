package handler

import (
	"fmt"
	"testing"

	"github.com/olivere/flipper/internal/display"
)

func TestImageCacheEvictsOldestOnly(t *testing.T) {
	c := NewImageCache()
	for i := range maxCacheEntries + 1 {
		c.Set(&display.Result{Filename: fmt.Sprintf("img-%d.png", i)})
	}

	if got := c.GetByFilename("img-0.png"); got != nil {
		t.Error("expected oldest entry to be evicted")
	}
	// Everything else — most importantly the newest entry, whose URL a
	// device was just handed — must survive.
	for i := 1; i <= maxCacheEntries; i++ {
		name := fmt.Sprintf("img-%d.png", i)
		if c.GetByFilename(name) == nil {
			t.Errorf("expected %s to still be cached", name)
		}
	}
}

func TestImageCacheDuplicateSet(t *testing.T) {
	c := NewImageCache()
	r := &display.Result{Filename: "same.png"}
	for range maxCacheEntries * 2 {
		c.Set(r)
	}
	if c.GetByFilename("same.png") == nil {
		t.Error("expected entry to be cached")
	}
	if n := len(c.order); n != 1 {
		t.Errorf("expected 1 order entry, got %d", n)
	}
}
