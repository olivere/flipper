package weather

import (
	"math"

	"github.com/fogleman/gg"
)

// drawIcon draws a weather icon for the given WMO code centered at (cx, cy).
// The size parameter controls the overall icon size.
func drawIcon(dc *gg.Context, code int, cx, cy, size float64) {
	switch {
	case code == 0:
		drawSun(dc, cx, cy, size)
	case code >= 1 && code <= 2:
		drawPartlyCloudy(dc, cx, cy, size)
	case code == 3:
		drawCloud(dc, cx, cy, size)
	case code >= 45 && code <= 48:
		drawFog(dc, cx, cy, size)
	case code >= 51 && code <= 57:
		drawDrizzle(dc, cx, cy, size)
	case code >= 61 && code <= 67, code >= 80 && code <= 82:
		drawRain(dc, cx, cy, size)
	case code >= 71 && code <= 77, code >= 85 && code <= 86:
		drawSnow(dc, cx, cy, size)
	case code >= 95 && code <= 99:
		drawThunder(dc, cx, cy, size)
	default:
		drawCloud(dc, cx, cy, size)
	}
}

// drawSun draws a sun: filled circle with radiating lines.
func drawSun(dc *gg.Context, cx, cy, size float64) {
	r := size * 0.3
	rayLen := size * 0.25
	rayGap := size * 0.08

	// Rays
	dc.SetLineWidth(math.Max(1, size*0.06))
	for i := range 8 {
		angle := float64(i) * math.Pi / 4
		x1 := cx + (r+rayGap)*math.Cos(angle)
		y1 := cy + (r+rayGap)*math.Sin(angle)
		x2 := cx + (r+rayGap+rayLen)*math.Cos(angle)
		y2 := cy + (r+rayGap+rayLen)*math.Sin(angle)
		dc.DrawLine(x1, y1, x2, y2)
		dc.Stroke()
	}

	// Center circle
	dc.DrawCircle(cx, cy, r)
	dc.Fill()
}

// drawCloud draws a cloud shape from overlapping circles.
func drawCloud(dc *gg.Context, cx, cy, size float64) {
	drawCloudShape(dc, cx, cy, size, 1.0)
}

// drawCloudShape draws a cloud at the given position and relative scale.
func drawCloudShape(dc *gg.Context, cx, cy, size, rel float64) {
	s := size * rel
	// Base ellipse
	dc.DrawEllipse(cx, cy+s*0.1, s*0.45, s*0.22)
	dc.Fill()
	// Left bump
	dc.DrawCircle(cx-s*0.22, cy-s*0.05, s*0.2)
	dc.Fill()
	// Right bump
	dc.DrawCircle(cx+s*0.18, cy-s*0.08, s*0.22)
	dc.Fill()
	// Top bump
	dc.DrawCircle(cx-s*0.02, cy-s*0.18, s*0.24)
	dc.Fill()
}

// drawPartlyCloudy draws a small sun peeking out behind a cloud.
func drawPartlyCloudy(dc *gg.Context, cx, cy, size float64) {
	// Sun offset to upper-right, behind cloud
	drawSun(dc, cx+size*0.2, cy-size*0.15, size*0.55)
	// Cloud in front, slightly lower-left
	// White outline to cut into sun
	dc.SetLineWidth(size * 0.08)
	dc.SetHexColor("ffffff")
	drawCloudOutline(dc, cx-size*0.1, cy+size*0.12, size, 0.8)
	dc.Stroke()
	dc.SetHexColor("000000")
	drawCloudShape(dc, cx-size*0.1, cy+size*0.12, size, 0.8)
}

// drawCloudOutline traces the cloud shape without filling.
func drawCloudOutline(dc *gg.Context, cx, cy, size, rel float64) {
	s := size * rel
	dc.DrawEllipse(cx, cy+s*0.1, s*0.45, s*0.22)
	dc.DrawCircle(cx-s*0.22, cy-s*0.05, s*0.2)
	dc.DrawCircle(cx+s*0.18, cy-s*0.08, s*0.22)
	dc.DrawCircle(cx-s*0.02, cy-s*0.18, s*0.24)
}

// drawRain draws a cloud with rain lines below it.
func drawRain(dc *gg.Context, cx, cy, size float64) {
	cloudY := cy - size*0.15
	drawCloudShape(dc, cx, cloudY, size, 0.7)

	// Rain lines
	dc.SetLineWidth(math.Max(1, size*0.05))
	dropTop := cloudY + size*0.2
	dropLen := size * 0.18
	for _, dx := range []float64{-0.2, 0, 0.2} {
		x := cx + size*dx
		dc.DrawLine(x, dropTop, x-size*0.05, dropTop+dropLen)
		dc.Stroke()
	}
}

// drawDrizzle draws a cloud with small dots below it.
func drawDrizzle(dc *gg.Context, cx, cy, size float64) {
	cloudY := cy - size*0.15
	drawCloudShape(dc, cx, cloudY, size, 0.7)

	// Small dots
	dotR := math.Max(1, size*0.04)
	dropY := cloudY + size*0.25
	for _, dx := range []float64{-0.15, 0.05, 0.2} {
		dc.DrawCircle(cx+size*dx, dropY, dotR)
		dc.Fill()
	}
}

// drawSnow draws a cloud with snowflake dots below it.
func drawSnow(dc *gg.Context, cx, cy, size float64) {
	cloudY := cy - size*0.15
	drawCloudShape(dc, cx, cloudY, size, 0.7)

	// Snowflake dots in two rows
	dotR := math.Max(1, size*0.04)
	for row, dy := range []float64{0.22, 0.35} {
		offsets := []float64{-0.18, 0, 0.18}
		if row == 1 {
			offsets = []float64{-0.09, 0.09}
		}
		for _, dx := range offsets {
			dc.DrawCircle(cx+size*dx, cloudY+size*dy, dotR)
			dc.Fill()
		}
	}
}

// drawThunder draws a cloud with a lightning bolt below it.
func drawThunder(dc *gg.Context, cx, cy, size float64) {
	cloudY := cy - size*0.18
	drawCloudShape(dc, cx, cloudY, size, 0.7)

	// Lightning bolt
	dc.SetLineWidth(math.Max(1.5, size*0.07))
	bx := cx - size*0.02
	by := cloudY + size*0.18
	dc.MoveTo(bx, by)
	dc.LineTo(bx-size*0.08, by+size*0.18)
	dc.LineTo(bx+size*0.05, by+size*0.18)
	dc.LineTo(bx-size*0.05, by+size*0.38)
	dc.Stroke()
}

// drawFog draws horizontal dashed lines representing fog.
func drawFog(dc *gg.Context, cx, cy, size float64) {
	dc.SetLineWidth(math.Max(1, size*0.06))
	lineW := size * 0.45
	for i, dy := range []float64{-0.2, -0.05, 0.1, 0.25} {
		// Alternate offset for visual interest
		offset := 0.0
		if i%2 == 1 {
			offset = size * 0.08
		}
		y := cy + size*dy
		dc.DrawLine(cx-lineW+offset, y, cx+lineW+offset, y)
		dc.Stroke()
	}
}
