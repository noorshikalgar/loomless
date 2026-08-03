package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Fully opaque: color.RGBA is alpha-premultiplied, so R/G/B must each be
// <= A — a translucent red like {255, 59, 48, 230} is an invalid
// premultiplied value (R > A) and silently mangles into a wrong color when
// the PNG encoder un-premultiplies it. Opaque sidesteps the invariant
// entirely (premultiplied == straight when A is 255).
var highlightColor = color.RGBA{R: 255, G: 59, B: 48, A: 255} // macOS system red
var badgeTextColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}

const (
	annotationBorderThickness = 3.0
	annotationBadgeRadius     = 11.0
)

// Annotation is the subset of session.Annotation this package needs —
// duplicated rather than imported to avoid a capture->session dependency
// (session already imports capture for Redact/RenderAnnotations; the
// reverse would cycle). Field order/names match session.Annotation.
type Annotation struct {
	Number int
	X, Y   float64
	W, H   float64
}

// PixelSize returns a PNG's width and height in pixels, decoding just the
// header rather than the full image — used to convert point-space
// coordinates (e.g. a click position relative to a window, in the
// AppleScript/System Events coordinate system) into the image's own pixel
// space, which differs by the display's backing scale factor on Retina.
func PixelSize(pngData []byte) (width, height int, err error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return 0, 0, fmt.Errorf("decode image config: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// RenderAnnotations draws every annotation's rectangle border plus a
// numbered badge onto a *copy* of the image — used at export time
// (internal/export) to flatten the editor's live overlay into a static
// picture. The stored screenshot itself is never touched by this; per
// decision #18, annotations are presentation-time data layered on top of
// clean capture-time pixels, unlike redaction.
func RenderAnnotations(pngData []byte, annotations []Annotation) ([]byte, error) {
	if len(annotations) == 0 {
		return pngData, nil
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	for _, a := range annotations {
		drawRectBorder(rgba, a.X, a.Y, a.X+a.W, a.Y+a.H, annotationBorderThickness, highlightColor)
		drawFilledCircle(rgba, a.X, a.Y, annotationBadgeRadius, highlightColor)
		drawCenteredNumber(rgba, a.X, a.Y, a.Number)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, rgba); err != nil {
		return nil, fmt.Errorf("encode annotated image: %w", err)
	}
	return out.Bytes(), nil
}

// Redact permanently fills a rectangular region with solid black, given in
// the image's own pixel coordinates. Unlike annotations, this mutates the
// image on disk (via session.EditImage) — solid fill, not blur, because
// per docs/product/06-editing-and-versioning.md's redaction carve-out the
// covered content must actually be unrecoverable, not just visually
// obscured.
func Redact(pngData []byte, x, y, w, h float64) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	fillRectSolid(rgba, x, y, x+w, y+h, color.RGBA{R: 0, G: 0, B: 0, A: 255})

	var out bytes.Buffer
	if err := png.Encode(&out, rgba); err != nil {
		return nil, fmt.Errorf("encode edited image: %w", err)
	}
	return out.Bytes(), nil
}

// fillRectSolid fills every pixel in [minX,maxX) x [minY,maxY) — used for
// Redact, where the interior must actually be overwritten, not just
// outlined.
func fillRectSolid(img *image.RGBA, minXf, minYf, maxXf, maxYf float64, c color.RGBA) {
	minX, minY := int(math.Round(minXf)), int(math.Round(minYf))
	maxX, maxY := int(math.Round(maxXf)), int(math.Round(maxYf))
	b := img.Bounds()
	for y := minY; y < maxY; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := minX; x < maxX; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			img.Set(x, y, c)
		}
	}
}

// drawRectBorder draws only the border (thickness px) of
// [minX,maxX) x [minY,maxY) — used for annotation outlines, where the
// interior must stay visible (that's the whole point of highlighting
// something rather than hiding it).
func drawRectBorder(img *image.RGBA, minXf, minYf, maxXf, maxYf, thickness float64, c color.RGBA) {
	minX, minY := int(math.Round(minXf)), int(math.Round(minYf))
	maxX, maxY := int(math.Round(maxXf)), int(math.Round(maxYf))
	t := int(math.Round(thickness))
	b := img.Bounds()
	for y := minY; y < maxY; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := minX; x < maxX; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			onBorder := x < minX+t || x >= maxX-t || y < minY+t || y >= maxY-t
			if onBorder {
				img.Set(x, y, c)
			}
		}
	}
}

// drawFilledCircle fills a solid disc of the given radius centered at
// (cx, cy) — the badge's circular background.
func drawFilledCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	minX := int(math.Floor(cx - r))
	maxX := int(math.Ceil(cx + r))
	minY := int(math.Floor(cy - r))
	maxY := int(math.Ceil(cy + r))
	b := img.Bounds()
	for y := minY; y <= maxY; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := minX; x <= maxX; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			if math.Hypot(float64(x)-cx, float64(y)-cy) <= r {
				img.Set(x, y, c)
			}
		}
	}
}

// drawCenteredNumber draws n's decimal digits centered at (cx, cy) using
// a built-in bitmap font (golang.org/x/image/font/basicfont) — no font
// file to bundle, which matters for a project that otherwise has zero
// asset dependencies (see decision #12's reasoning for the same choice in
// the PDF exporter).
func drawCenteredNumber(img *image.RGBA, cx, cy float64, n int) {
	label := strconv.Itoa(n)
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(badgeTextColor),
		Face: face,
	}
	width := float64(d.MeasureString(label)) / 64
	metrics := face.Metrics()
	ascent := float64(metrics.Ascent) / 64
	descent := float64(metrics.Descent) / 64

	x := cx - width/2
	y := cy + (ascent-descent)/2
	d.Dot = fixed.Point26_6{X: fixed.I(int(math.Round(x))), Y: fixed.I(int(math.Round(y)))}
	d.DrawString(label)
}
