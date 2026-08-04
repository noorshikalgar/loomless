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
	"strings"

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

// DefaultAnnotationColor is the "#rrggbb" hex string form of
// highlightColor — exported so callers (session.Annotation's doc
// comment, the editor's color-picker default) can refer to the same
// value by name instead of duplicating the literal.
const DefaultAnnotationColor = "#ff3b30"

const (
	annotationBorderThickness = 3.0
	annotationBadgeRadius     = 11.0
	freehandStrokeThickness   = 4.0
)

// Annotation is the subset of session.Annotation this package needs —
// duplicated rather than imported to avoid a capture->session dependency
// (session already imports capture for Redact/RenderAnnotations; the
// reverse would cycle). Field order/names match session.Annotation,
// with one deliberate difference: Annotated here is a plain bool, not a
// *bool — session.Annotation's nil-means-legacy-true convention exists
// entirely to keep old session.json files loading correctly, and
// export/image.go's toCaptureAnnotations is the one place that
// resolves it (via session.Annotation.IsAnnotated()) before values ever
// reach this package, so by the time RenderAnnotations sees Annotated,
// false always genuinely means "don't draw the badge."
type Annotation struct {
	Number int
	X, Y   float64
	W, H   float64
	Color  string // "#rrggbb", or "" for the default (see parseAnnotationColor)
	// Points holds a path shape's vertices: every point for "freehand",
	// exactly two (start, end) for "line"/"arrow". Empty for a box shape
	// ("rect"/"circle" — X/Y/W/H is the drawn rectangle instead). For
	// every kind, X/Y/W/H is also the shape's bounding box, used for the
	// badge's position regardless of kind.
	Points []Point
	// Kind selects how the shape renders: "" or "rect" (rectangle
	// border), "circle" (ellipse inscribed in X/Y/W/H), "line" or
	// "arrow" (segment through Points[0]/Points[1] — arrow adds an
	// arrowhead), or "freehand" (path through every Points entry). See
	// shapeKind for how "" + non-empty Points (every annotation kind
	// this package could produce before Kind existed) resolves.
	Kind string
	// Annotated is whether the numbered badge is drawn at all — see
	// this struct's own doc comment for why it's a plain bool here.
	Annotated bool
}

// shapeKind resolves an annotation's effective shape kind, folding in
// the one backward-compatibility rule needed for data saved before Kind
// existed: Kind == "" with non-empty Points was always a freehand
// stroke (the only thing empty-Kind-with-Points could have meant), and
// Kind == "" with no Points was always a rectangle.
func shapeKind(a Annotation) string {
	if a.Kind != "" {
		return a.Kind
	}
	if len(a.Points) > 0 {
		return "freehand"
	}
	return "rect"
}

// Point mirrors session.Point — duplicated for the same reason
// Annotation itself is (see above): avoiding a capture<->session import
// cycle.
type Point struct {
	X, Y float64
}

// parseAnnotationColor turns a "#rrggbb" (or "rrggbb", case-insensitive)
// hex string into an opaque color.RGBA, falling back to the default
// highlight color for anything empty or malformed — an annotation's
// stored color is user-supplied free text that flows all the way from a
// browser's <input type="color"> through an HTTP request body, so a
// missing or garbled value must degrade to something reasonable, never
// panic or corrupt rendering.
func parseAnnotationColor(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return highlightColor
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return highlightColor
	}
	return color.RGBA{
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
		A: 255,
	}
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
		c := parseAnnotationColor(a.Color)
		switch shapeKind(a) {
		case "circle":
			drawEllipseBorder(rgba, a.X+a.W/2, a.Y+a.H/2, a.W/2, a.H/2, annotationBorderThickness, c)
		case "line":
			drawPolyline(rgba, a.Points, freehandStrokeThickness, c)
		case "arrow":
			drawPolyline(rgba, a.Points, freehandStrokeThickness, c)
			if len(a.Points) >= 2 {
				drawArrowhead(rgba, a.Points[0], a.Points[len(a.Points)-1], freehandStrokeThickness, c)
			}
		case "freehand":
			drawPolyline(rgba, a.Points, freehandStrokeThickness, c)
		default: // "rect"
			drawRectBorder(rgba, a.X, a.Y, a.X+a.W, a.Y+a.H, annotationBorderThickness, c)
		}
		// A shape's badge is opt-in (decision #30) — most drawn shapes
		// are plain markup, not part of a numbered callout sequence.
		if a.Annotated {
			drawFilledCircle(rgba, a.X, a.Y, annotationBadgeRadius, c)
			drawCenteredNumber(rgba, a.X, a.Y, a.Number)
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, rgba); err != nil {
		return nil, fmt.Errorf("encode annotated image: %w", err)
	}
	return out.Bytes(), nil
}

// Crop returns a new image containing only the given rectangular region
// (in the image's own pixel coordinates), clamped to the image's actual
// bounds — a crop rectangle drawn slightly past an edge (a very
// plausible off-by-a-few-pixels drag, especially near a corner) clips to
// what actually exists rather than erroring or padding with blank
// pixels. Unlike Redact, this never mutates the caller's stored file
// (see session.Step.Crop's doc comment) — it's called fresh at export
// time against the original bytes every time.
func Crop(pngData []byte, x, y, w, h float64) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	minX, minY := int(math.Round(x)), int(math.Round(y))
	maxX, maxY := int(math.Round(x+w)), int(math.Round(y+h))
	if minX < bounds.Min.X {
		minX = bounds.Min.X
	}
	if minY < bounds.Min.Y {
		minY = bounds.Min.Y
	}
	if maxX > bounds.Max.X {
		maxX = bounds.Max.X
	}
	if maxY > bounds.Max.Y {
		maxY = bounds.Max.Y
	}
	if minX >= maxX || minY >= maxY {
		return nil, fmt.Errorf("crop rectangle is entirely outside the image")
	}

	cropped := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{X: minX, Y: minY}, draw.Src)

	var out bytes.Buffer
	if err := png.Encode(&out, cropped); err != nil {
		return nil, fmt.Errorf("encode cropped image: %w", err)
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

// drawPolyline draws a connected, thick, round-jointed stroke through
// points — a freehand annotation's path. Rather than a dedicated
// thick-line rasterizer, this stamps a filled circle (drawFilledCircle,
// already correct and bounds-safe) at closely-spaced steps along each
// segment; consecutive circles overlap enough to read as one continuous
// stroke with naturally rounded joints at every vertex, for less new
// code than a proper polygon-based thick line and no visible difference
// at this stroke width.
func drawPolyline(img *image.RGBA, points []Point, thickness float64, c color.RGBA) {
	r := thickness / 2
	if len(points) == 1 {
		drawFilledCircle(img, points[0].X, points[0].Y, r, c)
		return
	}
	for i := 0; i < len(points)-1; i++ {
		a, b := points[i], points[i+1]
		dist := math.Hypot(b.X-a.X, b.Y-a.Y)
		// Half-radius spacing keeps consecutive stamps overlapping
		// enough to look like a solid stroke rather than a dotted line.
		steps := int(math.Ceil(dist / (r * 0.5)))
		if steps < 1 {
			steps = 1
		}
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			drawFilledCircle(img, a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t, r, c)
		}
	}
}

// drawEllipseBorder strokes the outline of the ellipse inscribed in a
// bounding box centered at (cx, cy) with radii (rx, ry) — sampled via
// the same overlapping-stamped-circles technique drawPolyline already
// uses for a stroke, walking the parametric ellipse and stamping a
// filled circle at each angle, rather than a distance-field approach:
// rx and ry can differ (a non-uniform ellipse), and there's no single
// "distance from the boundary" that works the same in both axes without
// extra math this sidesteps entirely by construction.
func drawEllipseBorder(img *image.RGBA, cx, cy, rx, ry, thickness float64, c color.RGBA) {
	if rx <= 0 || ry <= 0 {
		return
	}
	r := thickness / 2
	// Ramanujan's circumference approximation picks a step count that
	// keeps stamped circles overlapping all the way around regardless of
	// how elongated the ellipse is — the same spacing goal drawPolyline
	// uses along a straight segment, just applied around a curve.
	h := math.Pow(rx-ry, 2) / math.Pow(rx+ry, 2)
	circumference := math.Pi * (rx + ry) * (1 + 3*h/(10+math.Sqrt(4-3*h)))
	steps := int(math.Ceil(circumference / (r * 0.5)))
	if steps < 32 {
		steps = 32
	}
	for i := 0; i <= steps; i++ {
		theta := 2 * math.Pi * float64(i) / float64(steps)
		drawFilledCircle(img, cx+rx*math.Cos(theta), cy+ry*math.Sin(theta), r, c)
	}
}

// drawArrowhead draws two short wings at `to`, angled back toward
// `from` — composed from drawPolyline itself (each wing is just another
// two-point stroke) rather than a dedicated triangle rasterizer, the
// same "reuse the existing stroke primitive" choice drawPolyline made
// for drawFilledCircle.
func drawArrowhead(img *image.RGBA, from, to Point, thickness float64, c color.RGBA) {
	const wingAngle = math.Pi / 7 // ~25.7 degrees off the shaft
	const wingLength = 14.0

	dx, dy := to.X-from.X, to.Y-from.Y
	if math.Hypot(dx, dy) < 1 {
		return
	}
	angle := math.Atan2(dy, dx)
	for _, sign := range [2]float64{1, -1} {
		wingDir := angle + math.Pi - sign*wingAngle
		wing := Point{X: to.X + wingLength*math.Cos(wingDir), Y: to.Y + wingLength*math.Sin(wingDir)}
		drawPolyline(img, []Point{to, wing}, thickness, c)
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
