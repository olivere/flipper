package display

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
)

const (
	bmpFileHeaderSize = 14
	bmpInfoHeaderSize = 40
	bmpPaletteSize    = 2 * 4
	bmpPixelOffset    = bmpFileHeaderSize + bmpInfoHeaderSize + bmpPaletteSize
	bmpPixelsPerMeter = 2835 // 72 DPI
)

// encodeBMP1 writes img as an uncompressed 1-bit BMP with a
// BITMAPINFOHEADER, the format the TRMNL OG firmware decodes. Palette
// index 0 maps to bit 0. Rows are stored bottom-up and padded to a
// multiple of 4 bytes, as the BMP format requires.
func encodeBMP1(w io.Writer, img *image.Paletted) error {
	if len(img.Palette) != 2 {
		return fmt.Errorf("display/encodeBMP1: need a 2-color palette, got %d colors", len(img.Palette))
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width <= 0 || height <= 0 {
		return errors.New("display/encodeBMP1: empty image")
	}

	rowSize := (width + 31) / 32 * 4
	imageSize := rowSize * height
	buf := make([]byte, bmpPixelOffset+imageSize)

	le := binary.LittleEndian
	buf[0], buf[1] = 'B', 'M'
	le.PutUint32(buf[2:], uint32(len(buf)))
	le.PutUint32(buf[10:], bmpPixelOffset)

	info := buf[bmpFileHeaderSize:]
	le.PutUint32(info[0:], bmpInfoHeaderSize)
	le.PutUint32(info[4:], uint32(width))
	le.PutUint32(info[8:], uint32(height)) // positive height: bottom-up rows
	le.PutUint16(info[12:], 1)             // planes
	le.PutUint16(info[14:], 1)             // bits per pixel
	le.PutUint32(info[20:], uint32(imageSize))
	le.PutUint32(info[24:], bmpPixelsPerMeter)
	le.PutUint32(info[28:], bmpPixelsPerMeter)
	le.PutUint32(info[32:], 2) // colors used

	pal := buf[bmpFileHeaderSize+bmpInfoHeaderSize:]
	for i, c := range img.Palette {
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		pal[i*4+0], pal[i*4+1], pal[i*4+2] = n.B, n.G, n.R
	}

	pix := buf[bmpPixelOffset:]
	for y := range height {
		row := pix[(height-1-y)*rowSize:]
		src := img.Pix[img.PixOffset(b.Min.X, b.Min.Y+y):]
		for x := range width {
			if src[x] != 0 {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
	}

	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("display/encodeBMP1: %w", err)
	}
	return nil
}
