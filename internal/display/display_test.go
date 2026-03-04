package display

import (
	"image"
	"image/color"
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

func TestProcessOriginal(t *testing.T) {
	p := NewPipeline()
	src := testImage(1024, 768)

	result, err := p.Process(src, ProfileOriginal, "fit")
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

func TestProcessOriginalFill(t *testing.T) {
	p := NewPipeline()
	src := testImage(1024, 768)

	result, err := p.Process(src, ProfileOriginal, "fill")
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
}

func TestDeterministicHash(t *testing.T) {
	p := NewPipeline()
	src := testImage(200, 200)

	r1, err := p.Process(src, ProfileOriginal, "fit")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := p.Process(src, ProfileOriginal, "fit")
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
		{800, 480, ProfileOriginal},
		{1872, 1404, ProfileX},
		{0, 0, ProfileOriginal},   // default
		{100, 100, ProfileOriginal}, // unknown defaults to original
	}
	for _, tt := range tests {
		got := DetectProfile(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("DetectProfile(%d, %d) = %v, want %v", tt.w, tt.h, got, tt.want)
		}
	}
}
