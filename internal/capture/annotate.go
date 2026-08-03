package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// highlightRadiusPts and highlightThicknessPts are in the same point unit
// as WindowInfo bounds — Highlight converts to pixels itself using the
// screenshot's actual size, so the ring looks the same size regardless of
// display scale factor (Retina vs standard).
const (
	highlightRadiusPts    = 14.0
	highlightThicknessPts = 3.0
)

// Fully opaque: color.RGBA is alpha-premultiplied, so R/G/B must each be
// <= A — a translucent red like {255, 59, 48, 230} is an invalid
// premultiplied value (R > A) and silently mangles into a wrong color when
// the PNG encoder un-premultiplies it. Opaque sidesteps the invariant
// entirely (premultiplied == straight when A is 255).
var highlightColor = color.RGBA{R: 255, G: 59, B: 48, A: 255} // macOS system red

// Highlight draws a ring marker at (relX, relY) — the click point relative
// to the captured window's top-left origin, in points — onto a PNG
// screenshot. The screenshot's pixel dimensions vs. windowWidthPts give the
// display's backing scale factor, so the marker lands correctly on both
// standard and Retina captures.
func Highlight(pngData []byte, relX, relY, windowWidthPts, windowHeightPts float64) ([]byte, error) {
	if windowWidthPts <= 0 || windowHeightPts <= 0 {
		return pngData, nil
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	bounds := img.Bounds()
	scale := float64(bounds.Dx()) / windowWidthPts

	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	cx := float64(bounds.Min.X) + relX*scale
	cy := float64(bounds.Min.Y) + relY*scale
	r := highlightRadiusPts * scale
	t := highlightThicknessPts * scale
	drawRing(rgba, cx, cy, r, t, highlightColor)

	var out bytes.Buffer
	if err := png.Encode(&out, rgba); err != nil {
		return nil, fmt.Errorf("encode annotated screenshot: %w", err)
	}
	return out.Bytes(), nil
}

// drawRing fills a circular ring (outer radius r, given thickness) centered
// at (cx, cy). A plain distance check rather than a proper anti-aliased
// stroke — sufficient for a "here's roughly where I clicked" marker.
func drawRing(img *image.RGBA, cx, cy, r, t float64, c color.RGBA) {
	inner := r - t/2
	outer := r + t/2
	minX := int(math.Floor(cx - outer))
	maxX := int(math.Ceil(cx + outer))
	minY := int(math.Floor(cy - outer))
	maxY := int(math.Ceil(cy + outer))

	b := img.Bounds()
	for y := minY; y <= maxY; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := minX; x <= maxX; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			if d >= inner && d <= outer {
				img.Set(x, y, c)
			}
		}
	}
}
