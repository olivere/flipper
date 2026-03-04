// Package display implements the image pipeline for TRMNL e-ink devices.
//
// The pipeline converts a source image through these stages:
//
//  1. Resize — fit or fill to the target device dimensions
//  2. Grayscale — convert to 8-bit grayscale
//  3. Contrast + sharpen — boost readability on e-ink
//  4. Dither — Floyd-Steinberg to 1-bit BW (original) or 4-level gray (X)
//  5. Encode — BMP for original devices, PNG for TRMNL X
//
// Device profiles (original 800×480, X 1872×1404) determine the output
// format and dither palette.
package display
