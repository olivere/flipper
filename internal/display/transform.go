package display

import (
	"image"
	"math"

	"golang.org/x/image/draw"
)

// scaleOntoWhite scales src into a width×height canvas filled with white
// and composites it over the white, which removes any transparency.
//
// "fill" scales src to cover the canvas and crops the overflow evenly
// on both sides. Any other mode fits src inside the canvas and centers
// it; a source that already fits is not upscaled.
func scaleOntoWhite(src image.Image, width, height int, scaling string) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)

	sb := src.Bounds()
	srcW, srcH := sb.Dx(), sb.Dy()
	if srcW <= 0 || srcH <= 0 {
		return dst
	}

	srcRect, dstRect := sb, dst.Bounds()
	if scaling == "fill" {
		srcRect = centerCrop(sb, float64(width)/float64(height))
	} else {
		dstRect = fitRect(srcW, srcH, width, height)
	}

	if srcRect.Size() == dstRect.Size() {
		draw.Draw(dst, dstRect, src, srcRect.Min, draw.Over)
	} else {
		draw.CatmullRom.Scale(dst, dstRect, src, srcRect, draw.Over, nil)
	}
	return dst
}

// centerCrop returns the largest rectangle inside b with the given
// width/height aspect ratio, centered in b.
func centerCrop(b image.Rectangle, aspect float64) image.Rectangle {
	w, h := b.Dx(), b.Dy()
	if float64(w)/float64(h) < aspect {
		cropH := max(1, int(math.Round(float64(w)/aspect)))
		y0 := b.Min.Y + (h-cropH)/2
		return image.Rect(b.Min.X, y0, b.Max.X, y0+cropH)
	}
	cropW := max(1, int(math.Round(float64(h)*aspect)))
	x0 := b.Min.X + (w-cropW)/2
	return image.Rect(x0, b.Min.Y, x0+cropW, b.Max.Y)
}

// fitRect returns where a srcW×srcH image lands when it is fitted into
// a width×height canvas without upscaling and centered.
func fitRect(srcW, srcH, width, height int) image.Rectangle {
	w, h := srcW, srcH
	if w > width || h > height {
		aspect := float64(srcW) / float64(srcH)
		if aspect > float64(width)/float64(height) {
			w, h = width, max(1, int(float64(width)/aspect))
		} else {
			w, h = max(1, int(float64(height)*aspect)), height
		}
	}
	x0, y0 := (width-w)/2, (height-h)/2
	return image.Rect(x0, y0, x0+w, y0+h)
}

// toGray converts an opaque image to luma using the Rec. 601 weights.
func toGray(src *image.RGBA) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		s := src.Pix[src.PixOffset(b.Min.X, y):]
		d := dst.Pix[dst.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			r, g, bl := float64(s[x*4]), float64(s[x*4+1]), float64(s[x*4+2])
			d[x] = uint8(0.299*r + 0.587*g + 0.114*bl + 0.5)
		}
	}
	return dst
}

// adjustContrast changes contrast in place around mid-gray. percentage
// is clamped to [-100, 100]: 0 leaves the image unchanged, -100 makes
// it flat gray and 100 thresholds it to black and white.
func adjustContrast(img *image.Gray, percentage float64) {
	v := (100 + min(max(percentage, -100), 100)) / 100
	var lut [256]uint8
	for i := range lut {
		f := float64(i) / 255
		switch {
		case v <= 1:
			lut[i] = clampByte((0.5 + (f-0.5)*v) * 255)
		case v < 2:
			lut[i] = clampByte((0.5 + (f-0.5)/(2-v)) * 255)
		default:
			lut[i] = uint8(f+0.5) * 255
		}
	}
	for i, p := range img.Pix {
		img.Pix[i] = lut[p]
	}
}

// sharpen returns an unsharp-masked copy of img: twice each pixel minus
// a Gaussian blur of radius sigma. A sigma of 0 or less returns a copy.
func sharpen(img *image.Gray, sigma float64) *image.Gray {
	dst := image.NewGray(img.Bounds())
	if sigma <= 0 {
		draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
		return dst
	}
	blurred := gaussianBlur(img, sigma)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for y := range h {
		for x := range w {
			p := float64(img.Pix[y*img.Stride+x])
			dst.Pix[y*dst.Stride+x] = clampByte(2*p - blurred[y*w+x])
		}
	}
	return dst
}

// gaussianBlur returns a separable Gaussian blur of img with a kernel
// radius of 3·sigma. Near the edges, the weights are renormalized over
// the pixels that exist, so borders don't darken.
func gaussianBlur(img *image.Gray, sigma float64) []float64 {
	radius := int(math.Ceil(sigma * 3))
	kernel := make([]float64, radius+1)
	for i := range kernel {
		x := float64(i)
		kernel[i] = math.Exp(-(x * x) / (2 * sigma * sigma))
	}

	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	at := func(x, y int) float64 { return float64(img.Pix[y*img.Stride+x]) }

	horiz := make([]float64, w*h)
	for y := range h {
		for x := range w {
			var sum, wsum float64
			for ix := max(0, x-radius); ix <= min(w-1, x+radius); ix++ {
				k := kernel[abs(x-ix)]
				sum += at(ix, y) * k
				wsum += k
			}
			horiz[y*w+x] = sum / wsum
		}
	}

	out := make([]float64, w*h)
	for y := range h {
		for x := range w {
			var sum, wsum float64
			for iy := max(0, y-radius); iy <= min(h-1, y+radius); iy++ {
				k := kernel[abs(y-iy)]
				sum += horiz[iy*w+x] * k
				wsum += k
			}
			out[y*w+x] = sum / wsum
		}
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func clampByte(v float64) uint8 {
	return uint8(min(max(math.Round(v), 0), 255))
}
