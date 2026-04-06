package layout

import (
	"embed"
	"fmt"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed fonts/Inter-Regular.ttf
//go:embed fonts/Inter-Bold.ttf
//go:embed fonts/JetBrainsMono-Regular.ttf
var fontsFS embed.FS

// FontStyle selects a font variant.
type FontStyle int

const (
	FontRegular FontStyle = iota
	FontBold
	FontMono
)

var (
	fontCache   = make(map[FontStyle]*truetype.Font)
	fontCacheMu sync.RWMutex
)

func fontPath(style FontStyle) string {
	switch style {
	case FontBold:
		return "fonts/Inter-Bold.ttf"
	case FontMono:
		return "fonts/JetBrainsMono-Regular.ttf"
	default:
		return "fonts/Inter-Regular.ttf"
	}
}

func loadFont(style FontStyle) (*truetype.Font, error) {
	fontCacheMu.RLock()
	if f, ok := fontCache[style]; ok {
		fontCacheMu.RUnlock()
		return f, nil
	}
	fontCacheMu.RUnlock()

	fontCacheMu.Lock()
	defer fontCacheMu.Unlock()

	// Re-check after upgrading the lock.
	if f, ok := fontCache[style]; ok {
		return f, nil
	}

	data, err := fontsFS.ReadFile(fontPath(style))
	if err != nil {
		return nil, fmt.Errorf("layout: read font: %w", err)
	}
	f, err := truetype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("layout: parse font: %w", err)
	}
	fontCache[style] = f
	return f, nil
}

// Face returns a font.Face for the given style and size in points.
func Face(style FontStyle, size float64) (font.Face, error) {
	f, err := loadFont(style)
	if err != nil {
		return nil, err
	}
	return truetype.NewFace(f, &truetype.Options{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	}), nil
}
