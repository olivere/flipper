package display

import (
	"image"
	"image/color"
	"testing"
)

func solid(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

// bbox returns the bounds of all pixels in img that are not white.
func bbox(img *image.RGBA) image.Rectangle {
	var r image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y) != (color.RGBA{255, 255, 255, 255}) {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func TestScaleOntoWhiteFit(t *testing.T) {
	tests := []struct {
		name      string
		srcW      int
		srcH      int
		wantBound image.Rectangle
	}{
		{"wider than canvas", 1600, 480, image.Rect(0, 120, 800, 360)},
		{"taller than canvas", 480, 960, image.Rect(280, 0, 520, 480)},
		{"smaller is not upscaled", 100, 50, image.Rect(350, 215, 450, 265)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scaleOntoWhite(solid(tt.srcW, tt.srcH, color.Black), 800, 480, "fit")
			if got.Bounds() != image.Rect(0, 0, 800, 480) {
				t.Fatalf("canvas bounds = %v", got.Bounds())
			}
			if b := bbox(got); b != tt.wantBound {
				t.Errorf("image placed at %v, want %v", b, tt.wantBound)
			}
		})
	}
}

func TestScaleOntoWhiteFillCropsCenter(t *testing.T) {
	// A 3:1 source with black thirds on the left and right and a gray
	// middle. Filling a 1:1 canvas keeps only the middle third.
	src := solid(300, 100, color.Black)
	for y := range 100 {
		for x := 100; x < 200; x++ {
			src.Set(x, y, color.Gray{0x80})
		}
	}
	got := scaleOntoWhite(src, 50, 50, "fill")
	for _, p := range []image.Point{{0, 0}, {49, 0}, {25, 25}, {0, 49}, {49, 49}} {
		if c := got.RGBAAt(p.X, p.Y); c != (color.RGBA{0x80, 0x80, 0x80, 0xff}) {
			t.Errorf("pixel %v = %v, want mid gray", p, c)
		}
	}
}

func TestScaleOntoWhiteFlattensTransparency(t *testing.T) {
	src := solid(10, 10, color.NRGBA{0, 0, 0, 0x80})
	got := scaleOntoWhite(src, 10, 10, "fit")
	// 50% black over white.
	if c := got.RGBAAt(5, 5); c.R < 0x7e || c.R > 0x81 || c.A != 0xff {
		t.Errorf("pixel = %v, want opaque mid gray", c)
	}
}

func TestToGrayUsesRec601Weights(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	src.SetRGBA(2, 0, color.RGBA{0, 0, 255, 255})
	got := toGray(src)
	want := []uint8{76, 150, 29}
	for i, w := range want {
		if got.Pix[i] != w {
			t.Errorf("pixel %d = %d, want %d", i, got.Pix[i], w)
		}
	}
}

func TestAdjustContrast(t *testing.T) {
	tests := []struct {
		percentage float64
		in, want   uint8
	}{
		{0, 100, 100},
		{20, 0, 0},
		{20, 255, 255},
		{20, 64, 48},   // (0.5 + (64/255-0.5)*1.25) * 255
		{20, 191, 207}, // (0.5 + (191/255-0.5)*1.25) * 255
		{-50, 0, 64},   // (0.5 - 0.5*0.5) * 255
		{100, 127, 0},
		{100, 128, 255},
	}
	for _, tt := range tests {
		img := image.NewGray(image.Rect(0, 0, 1, 1))
		img.Pix[0] = tt.in
		adjustContrast(img, tt.percentage)
		if img.Pix[0] != tt.want {
			t.Errorf("adjustContrast(%d, %v) = %d, want %d", tt.in, tt.percentage, img.Pix[0], tt.want)
		}
	}
}

func TestSharpen(t *testing.T) {
	flat := image.NewGray(image.Rect(0, 0, 8, 8))
	for i := range flat.Pix {
		flat.Pix[i] = 100
	}
	got := sharpen(flat, 0.5)
	for i, p := range got.Pix {
		if p != 100 {
			t.Fatalf("flat image changed at %d: %d", i, p)
		}
	}

	// A vertical edge from 64 to 192 overshoots on both sides.
	edge := image.NewGray(image.Rect(0, 0, 8, 1))
	for x := range 8 {
		edge.Pix[x] = 64
		if x >= 4 {
			edge.Pix[x] = 192
		}
	}
	got = sharpen(edge, 0.5)
	if got.Pix[3] >= 64 || got.Pix[4] <= 192 {
		t.Errorf("edge not sharpened: dark side %d, light side %d", got.Pix[3], got.Pix[4])
	}
	if got.Pix[0] != 64 || got.Pix[7] != 192 {
		t.Errorf("pixels away from the edge changed: %d, %d", got.Pix[0], got.Pix[7])
	}
}
