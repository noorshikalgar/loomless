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

// RedactStyleSolid fills the region with opaque black — genuinely
// unrecoverable, per docs/product/06-editing-and-versioning.md's redaction
// carve-out. This is the default and the only style that should be used for
// actual secrets (API keys, tokens, credentials).
const RedactStyleSolid = "solid"

// RedactStyleBlur obscures the region with a strong box blur instead of
// deleting it — content stays roughly shape/color-visible but individual
// details (text, faces) become illegible. This is NOT cryptographically
// irreversible the way solid fill is; it exists for cases where the user
// wants to obscure something (a face in a demo, a distracting background
// element) without needing pixel-level guarantees. Never used for the
// masking pipeline's automatic secret redaction — only for user-initiated
// manual redaction in the editor.
const RedactStyleBlur = "blur"

// Redact permanently mutates a rectangular region of the image, given in
// the image's own pixel coordinates, per the given style (solid or blur —
// see RedactStyleSolid/RedactStyleBlur). An empty style defaults to solid.
func Redact(pngData []byte, x, y, w, h float64, style string) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	switch style {
	case RedactStyleBlur:
		blurRect(rgba, x, y, x+w, y+h)
	default:
		fillRectSolid(rgba, x, y, x+w, y+h, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	}

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

// blurRect obscures [minXf,maxXf) x [minYf,maxYf) in place with a strong
// box blur. Three passes of box blur closely approximate a Gaussian blur
// (a well-known equivalence) while staying simple — no separate kernel
// math, just repeated axis-aligned averaging over a scratch buffer.
func blurRect(img *image.RGBA, minXf, minYf, maxXf, maxYf float64) {
	const radius = 8
	const passes = 3

	minX, minY := int(math.Round(minXf)), int(math.Round(minYf))
	maxX, maxY := int(math.Round(maxXf)), int(math.Round(maxYf))
	b := img.Bounds()
	if minX < b.Min.X {
		minX = b.Min.X
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}
	w, h := maxX-minX, maxY-minY
	if w <= 0 || h <= 0 {
		return
	}

	region := make([]color.RGBA, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			region[y*w+x] = img.RGBAAt(minX+x, minY+y)
		}
	}

	tmp := make([]color.RGBA, w*h)
	for pass := 0; pass < passes; pass++ {
		// Horizontal pass: region -> tmp.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				tmp[y*w+x] = averageRun(region, w, h, x, y, radius, 1, 0)
			}
		}
		// Vertical pass: tmp -> region.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				region[y*w+x] = averageRun(tmp, w, h, x, y, radius, 0, 1)
			}
		}
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(minX+x, minY+y, region[y*w+x])
		}
	}
}

// averageRun averages the pixels within radius of (x,y) along direction
// (dx,dy) — (1,0) for a horizontal run, (0,1) for a vertical run.
func averageRun(src []color.RGBA, w, h, x, y, radius, dx, dy int) color.RGBA {
	var rSum, gSum, bSum, aSum, n uint32
	for i := -radius; i <= radius; i++ {
		xx, yy := x+i*dx, y+i*dy
		if xx < 0 || xx >= w || yy < 0 || yy >= h {
			continue
		}
		c := src[yy*w+xx]
		rSum += uint32(c.R)
		gSum += uint32(c.G)
		bSum += uint32(c.B)
		aSum += uint32(c.A)
		n++
	}
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(rSum / n), G: uint8(gSum / n), B: uint8(bSum / n), A: uint8(aSum / n)}
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
