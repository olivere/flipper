package display

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/jsummers/gobmp"
	"github.com/makeworld-the-better-one/dither/v2"
)

// DeviceProfile describes a TRMNL device's display capabilities.
type DeviceProfile struct {
	Width  int
	Height int
}

var (
	ProfileOriginal = DeviceProfile{Width: 800, Height: 480}
	ProfileX        = DeviceProfile{Width: 1872, Height: 1404}
)

func DetectProfile(width, height int) DeviceProfile {
	if width == ProfileX.Width && height == ProfileX.Height {
		return ProfileX
	}
	return ProfileOriginal
}

// Pipeline processes images for TRMNL devices.
type Pipeline struct{}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// Result is a processed image ready to serve.
type Result struct {
	Data     []byte
	Hash     string
	Format   string // "bmp" or "png"
	Filename string
}

// Process converts a source image for the given device profile.
// Scaling is "fit" (default) or "fill".
func (p *Pipeline) Process(src image.Image, profile DeviceProfile, scaling string) (*Result, error) {
	// Resize
	var resized *image.NRGBA
	if scaling == "fill" {
		resized = imaging.Fill(src, profile.Width, profile.Height, imaging.Center, imaging.Lanczos)
	} else {
		resized = imaging.Fit(src, profile.Width, profile.Height, imaging.Lanczos)
	}

	// For "fit" mode, paste onto white canvas at center
	if scaling != "fill" {
		canvas := imaging.New(profile.Width, profile.Height, color.White)
		offsetX := (profile.Width - resized.Bounds().Dx()) / 2
		offsetY := (profile.Height - resized.Bounds().Dy()) / 2
		canvas = imaging.Paste(canvas, resized, image.Pt(offsetX, offsetY))
		resized = canvas
	}

	// Grayscale
	gray := imaging.Grayscale(resized)

	// Contrast +20%
	adjusted := imaging.AdjustContrast(gray, 20)

	// Sharpen
	sharpened := imaging.Sharpen(adjusted, 0.5)

	// Dither and encode based on profile
	if profile == ProfileX {
		return encodeX(sharpened)
	}
	return encodeOriginal(sharpened)
}

func encodeOriginal(img *image.NRGBA) (*Result, error) {
	// 1-bit black/white dithering
	palette := []color.Color{
		color.Black,
		color.White,
	}
	d := dither.NewDitherer(palette)
	d.Matrix = dither.FloydSteinberg

	dithered := d.DitherPaletted(img)

	// Encode as BMP (gobmp auto-detects 1-bit from 2-color palette)
	var buf bytes.Buffer
	if err := gobmp.Encode(&buf, dithered); err != nil {
		return nil, fmt.Errorf("encode bmp: %w", err)
	}

	data := buf.Bytes()
	hash := fmt.Sprintf("%x", sha256.Sum256(data))[:16]

	return &Result{
		Data:     data,
		Hash:     hash,
		Format:   "bmp",
		Filename: hash + ".bmp",
	}, nil
}

func encodeX(img *image.NRGBA) (*Result, error) {
	// 4-level grayscale dithering
	palette := []color.Color{
		color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, // #000
		color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff}, // #555
		color.NRGBA{R: 0xaa, G: 0xaa, B: 0xaa, A: 0xff}, // #aaa
		color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // #fff
	}
	d := dither.NewDitherer(palette)
	d.Matrix = dither.FloydSteinberg

	dithered := d.Dither(img)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, dithered); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	data := buf.Bytes()
	hash := fmt.Sprintf("%x", sha256.Sum256(data))[:16]

	return &Result{
		Data:     data,
		Hash:     hash,
		Format:   "png",
		Filename: hash + ".png",
	}, nil
}
