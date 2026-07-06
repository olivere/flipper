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

// ColorMode describes a device's color capabilities.
type ColorMode int

const (
	ColorBW    ColorMode = iota // 1-bit black & white (TRMNL OG)
	ColorGray4                  // 4-bit grayscale, 16 levels (TRMNL X)
)

// DeviceProfile describes a TRMNL device's display capabilities.
type DeviceProfile struct {
	Name      string
	Width     int
	Height    int
	ColorMode ColorMode
}

var (
	ProfileOG = DeviceProfile{Name: "OG", Width: 800, Height: 480, ColorMode: ColorBW}
	ProfileX  = DeviceProfile{Name: "X", Width: 1872, Height: 1404, ColorMode: ColorGray4}
)

// DetectProfile returns the device profile matching the given display
// dimensions. Unrecognized sizes default to ProfileOG (800×480).
func DetectProfile(width, height int) DeviceProfile {
	if width == ProfileX.Width && height == ProfileX.Height {
		return ProfileX
	}
	return ProfileOG
}

// Pipeline processes images for TRMNL devices.
type Pipeline struct{}

// NewPipeline returns a new image processing pipeline.
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

	// Flatten onto a white canvas: centers "fit" output and composites
	// transparency away — e-ink has no alpha, and the palette encoders
	// map transparent pixels to black.
	canvas := imaging.New(profile.Width, profile.Height, color.White)
	offsetX := (profile.Width - resized.Bounds().Dx()) / 2
	offsetY := (profile.Height - resized.Bounds().Dy()) / 2
	resized = imaging.Overlay(canvas, resized, image.Pt(offsetX, offsetY), 1.0)

	// Grayscale
	gray := imaging.Grayscale(resized)

	// Contrast +20%
	adjusted := imaging.AdjustContrast(gray, 20)

	// Sharpen
	sharpened := imaging.Sharpen(adjusted, 0.5)

	// Dither and encode based on profile
	if profile.ColorMode == ColorGray4 {
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
	// 16-level grayscale dithering, matching the panel's native depth.
	// DitherPaletted yields an image.Paletted, which the PNG encoder
	// writes as 4-bit palette data — the format the firmware expects
	// (and a fraction of the size of an RGBA PNG).
	palette := make([]color.Color, 16)
	for i := range palette {
		v := uint8(i * 0x11)
		palette[i] = color.NRGBA{R: v, G: v, B: v, A: 0xff}
	}
	d := dither.NewDitherer(palette)
	d.Matrix = dither.FloydSteinberg

	dithered := d.DitherPaletted(img)

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
