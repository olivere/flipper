package display

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Create a gradient pattern
	for y := range h {
		for x := range w {
			g := uint8((x + y) % 256)
			img.Set(x, y, color.RGBA{R: g, G: g, B: g, A: 255})
		}
	}
	return img
}

func TestProcessOG(t *testing.T) {
	p := NewPipeline()
	src := testImage(1024, 768)

	result, err := p.Process(src, ProfileOG, "fit")
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "bmp" {
		t.Errorf("expected bmp format, got %s", result.Format)
	}
	if result.Filename == "" {
		t.Error("expected non-empty filename")
	}
	if len(result.Data) == 0 {
		t.Error("expected non-empty data")
	}
	// BMP magic bytes
	if result.Data[0] != 'B' || result.Data[1] != 'M' {
		t.Errorf("expected BMP magic bytes, got %x %x", result.Data[0], result.Data[1])
	}
}

func TestProcessOGFill(t *testing.T) {
	p := NewPipeline()
	src := testImage(1024, 768)

	result, err := p.Process(src, ProfileOG, "fill")
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "bmp" {
		t.Errorf("expected bmp format, got %s", result.Format)
	}
	if result.Data[0] != 'B' || result.Data[1] != 'M' {
		t.Errorf("expected BMP magic bytes, got %x %x", result.Data[0], result.Data[1])
	}
}

func TestProcessX(t *testing.T) {
	p := NewPipeline()
	// Use a smaller source to keep test fast
	src := testImage(800, 600)

	result, err := p.Process(src, ProfileX, "fit")
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "png" {
		t.Errorf("expected png format, got %s", result.Format)
	}
	if result.Filename == "" {
		t.Error("expected non-empty filename")
	}
	// PNG magic bytes
	if result.Data[0] != 0x89 || result.Data[1] != 'P' {
		t.Errorf("expected PNG magic bytes, got %x %x", result.Data[0], result.Data[1])
	}

	// The X panel has 16 native gray levels; verify the output is a
	// paletted PNG using the full 16-level palette.
	decoded, err := png.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	paletted, ok := decoded.(*image.Paletted)
	if !ok {
		t.Fatalf("expected paletted PNG, got %T", decoded)
	}
	if len(paletted.Palette) != 16 {
		t.Errorf("expected 16-color palette, got %d", len(paletted.Palette))
	}
	used := make(map[uint8]bool)
	for _, idx := range paletted.Pix {
		used[idx] = true
	}
	if len(used) <= 4 {
		t.Errorf("expected gradient to use more than 4 gray levels, got %d", len(used))
	}
}

func TestDeterministicHash(t *testing.T) {
	p := NewPipeline()
	src := testImage(200, 200)

	r1, err := p.Process(src, ProfileOG, "fit")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := p.Process(src, ProfileOG, "fit")
	if err != nil {
		t.Fatal(err)
	}

	if r1.Hash != r2.Hash {
		t.Errorf("expected same hash for same input, got %s != %s", r1.Hash, r2.Hash)
	}
	if r1.Filename != r2.Filename {
		t.Errorf("expected same filename for same input, got %s != %s", r1.Filename, r2.Filename)
	}
}

func TestDetectProfile(t *testing.T) {
	tests := []struct {
		w, h    int
		want    DeviceProfile
	}{
		{800, 480, ProfileOG},
		{1872, 1404, ProfileX},
		{0, 0, ProfileOG},   // default
		{100, 100, ProfileOG}, // unknown defaults to original
	}
	for _, tt := range tests {
		got := DetectProfile(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("DetectProfile(%d, %d) = %v, want %v", tt.w, tt.h, got, tt.want)
		}
	}
}
