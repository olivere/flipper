package display

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/bmp"
)

func TestEncodeBMP1RoundTrip(t *testing.T) {
	palette := color.Palette{color.Black, color.White}
	// Widths around byte and 4-byte row boundaries exercise the padding,
	// and a non-zero origin exercises the bounds handling.
	rects := []image.Rectangle{
		image.Rect(0, 0, 800, 480),
		image.Rect(0, 0, 1, 1),
		image.Rect(0, 0, 7, 3),
		image.Rect(0, 0, 33, 5),
		image.Rect(10, 20, 75, 22),
	}
	for _, r := range rects {
		src := image.NewPaletted(r, palette)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				src.SetColorIndex(x, y, uint8((x*3+y)%2))
			}
		}

		var buf bytes.Buffer
		if err := encodeBMP1(&buf, src); err != nil {
			t.Fatalf("%v: encode: %v", r, err)
		}
		if bpp := binary.LittleEndian.Uint16(buf.Bytes()[28:]); bpp != 1 {
			t.Fatalf("%v: bits per pixel = %d, want 1", r, bpp)
		}

		decoded, err := bmp.Decode(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("%v: decode: %v", r, err)
		}
		if got, want := decoded.Bounds().Size(), r.Size(); got != want {
			t.Fatalf("%v: decoded size %v, want %v", r, got, want)
		}
		for y := range r.Dy() {
			for x := range r.Dx() {
				want := color.GrayModel.Convert(src.At(r.Min.X+x, r.Min.Y+y))
				got := color.GrayModel.Convert(decoded.At(x, y))
				if got != want {
					t.Fatalf("%v: pixel (%d,%d) = %v, want %v", r, x, y, got, want)
				}
			}
		}
	}
}

func TestEncodeBMP1RejectsWrongPalette(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.Gray{0x80}, color.White})
	if err := encodeBMP1(&bytes.Buffer{}, img); err == nil {
		t.Fatal("expected an error for a 3-color palette")
	}
}
