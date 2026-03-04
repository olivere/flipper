package static

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/olivere/flipper/internal/screen"

	_ "golang.org/x/image/bmp"
)

var supportedExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".bmp":  true,
}

// Screen serves images from a directory in lexicographic order.
type Screen struct {
	mu    sync.Mutex
	dir   string
	files []string
	index int
}

func New(dir string) (*Screen, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("static screen: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("static screen: %s is not a directory", dir)
	}
	s := &Screen{dir: dir}
	if err := s.scan(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Screen) Name() string { return "static" }

func (s *Screen) Render(_ context.Context, _ screen.RenderOpts) (image.Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.scan(); err != nil {
		return nil, err
	}
	if len(s.files) == 0 {
		return nil, fmt.Errorf("static screen: no images in %s", s.dir)
	}

	path := s.files[s.index%len(s.files)]
	s.index = (s.index + 1) % len(s.files)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("static screen: open %s: %w", path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("static screen: decode %s: %w", path, err)
	}
	return img, nil
}

func (s *Screen) scan() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("static screen: read dir: %w", err)
	}

	s.files = s.files[:0]
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if supportedExts[ext] {
			s.files = append(s.files, filepath.Join(s.dir, e.Name()))
		}
	}
	sort.Strings(s.files)

	if s.index >= len(s.files) {
		s.index = 0
	}
	return nil
}
