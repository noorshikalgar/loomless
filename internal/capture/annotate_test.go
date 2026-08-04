package capture

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
)

// solidTestImage builds a 100x100 PNG filled with fillColor, so any pixel
// found not matching it after an edit must be inside the edited region.
func solidTestImage(t *testing.T, fillColor color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: fillColor}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	return buf.Bytes()
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

func TestRedactFillsRegionSolidBlackAndLeavesRestUntouched(t *testing.T) {
	// A distinct, identifiable color so we can tell "still original" from
	// "redacted" from "something else went wrong" unambiguously.
	original := color.RGBA{R: 200, G: 100, B: 50, A: 255}
	src := solidTestImage(t, original)

	edited, err := Redact(src, 20, 20, 30, 30, RedactStyleSolid) // rect [20,50) x [20,50)
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	img := decodePNG(t, edited)

	inside := []image.Point{{25, 25}, {49, 49}, {20, 20}}
	for _, p := range inside {
		r, g, b, a := img.At(p.X, p.Y).RGBA()
		if r>>8 != 0 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Errorf("pixel %v inside redacted rect = (%d,%d,%d,%d), want solid opaque black", p, r>>8, g>>8, b>>8, a>>8)
		}
	}

	// The original color must be gone entirely inside the rect, and must
	// still be present everywhere outside it (redaction shouldn't leak
	// into or shrink beyond the requested region).
	outside := []image.Point{{0, 0}, {19, 19}, {50, 50}, {99, 99}, {19, 35}, {50, 35}}
	for _, p := range outside {
		r, g, b, a := img.At(p.X, p.Y).RGBA()
		got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
		if got != original {
			t.Errorf("pixel %v outside redacted rect = %v, want unchanged original %v", p, got, original)
		}
	}
}

// TestRedactBlurObscuresRegionButLeavesRestUntouched checks the blur style
// against a genuinely blurrable image (a hard-edged two-color split) —
// solidTestImage's single flat fill can't demonstrate blur at all, since
// averaging a uniform color with itself is a no-op. The region-outside
// check matters most here: unlike solid fill, blur reads neighboring
// pixels while writing, so a bug that leaks the box-blur read/write past
// the intended rect would otherwise go unnoticed.
func TestRedactBlurObscuresRegionButLeavesRestUntouched(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	left := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	right := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			c := left
			if x >= 50 {
				c = right
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	src := buf.Bytes()

	edited, err := Redact(src, 40, 10, 20, 20, RedactStyleBlur) // rect [40,60) x [10,30), straddles the color split
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	decoded := decodePNG(t, edited)

	// Right at the color boundary inside the blurred rect, blending must
	// have happened: neither pure left nor pure right color survives.
	r, g, b, _ := decoded.At(49, 20).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)}
	if got == (color.RGBA{R: 255, G: 0, B: 0}) || got == (color.RGBA{R: 0, G: 0, B: 255}) {
		t.Errorf("pixel (49,20) inside blurred rect = %v, want a blend of red and blue, not either pure color", got)
	}

	// Outside the rect, both flat-color regions must be completely
	// untouched — no bleed from the box blur's read window.
	outside := []struct {
		p    image.Point
		want color.RGBA
	}{
		{image.Point{10, 10}, left},
		{image.Point{90, 90}, right},
		{image.Point{39, 20}, left},  // just left of the rect's left edge
		{image.Point{60, 20}, right}, // just right of the rect's right edge
	}
	for _, tc := range outside {
		r, g, b, a := decoded.At(tc.p.X, tc.p.Y).RGBA()
		got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
		if got != tc.want {
			t.Errorf("pixel %v outside blurred rect = %v, want unchanged %v", tc.p, got, tc.want)
		}
	}
}

func TestRenderAnnotationsDrawsBorderAndBadgeLeavesRestUntouched(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	src := solidTestImage(t, original)

	ann := Annotation{Number: 7, X: 20, Y: 20, W: 50, H: 50, Annotated: true} // rect [20,70) x [20,70)
	edited, err := RenderAnnotations(src, []Annotation{ann})
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	img := decodePNG(t, edited)

	// The border (left edge, vertically centered — away from the badge,
	// which sits at the top-left corner) should be the highlight color.
	r, g, b, a := img.At(20, 45).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != highlightColor {
		t.Errorf("border pixel (20,45) = %v, want highlight color %v", got, highlightColor)
	}

	// The interior, away from both the border and the corner badge,
	// should be untouched — annotations mark something up, they don't
	// obscure it (that's what Redact is for).
	r, g, b, a = img.At(50, 50).RGBA()
	got = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != original {
		t.Errorf("interior pixel (50,50) = %v, want untouched original %v", got, original)
	}

	// The badge is a filled circle with the number drawn on top in white
	// — the exact center pixel can legitimately be either color depending
	// on where the digit's glyph strokes fall (confirmed: for "7" here,
	// dead center lands on the glyph, not the background), so check a
	// point near the circle's edge instead, which is inside the badge
	// but well clear of the glyph, and must be the solid badge color.
	r, g, b, a = img.At(20-9, 20).RGBA()
	got = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != highlightColor {
		t.Errorf("badge edge pixel (11,20) = %v, want highlight color %v", got, highlightColor)
	}
	// And the exact center must differ from the untouched background —
	// whichever of badge-color or glyph-white it is, something was drawn.
	r, g, b, a = img.At(20, 20).RGBA()
	got = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got == original {
		t.Errorf("badge center (20,20) = %v, want it to differ from untouched original %v", got, original)
	}
}

func TestRenderAnnotationsEmptyListReturnsInputUnchanged(t *testing.T) {
	src := solidTestImage(t, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	out, err := RenderAnnotations(src, nil)
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	if !bytes.Equal(src, out) {
		t.Error("RenderAnnotations with no annotations should return the input unchanged, got different bytes")
	}
}

// TestRenderAnnotationsUsesPerAnnotationColor confirms two annotations
// with different Color values on the same image actually render in
// different colors — the whole point of a color picker is that it's
// per-annotation, not a single image-wide setting.
func TestRenderAnnotationsUsesPerAnnotationColor(t *testing.T) {
	src := solidTestImage(t, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	blue := Annotation{Number: 1, X: 10, Y: 10, W: 30, H: 30, Color: "#1e90ff"}
	green := Annotation{Number: 2, X: 60, Y: 10, W: 30, H: 30, Color: "#00ff00"}
	edited, err := RenderAnnotations(src, []Annotation{blue, green})
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	img := decodePNG(t, edited)

	wantBlue := color.RGBA{R: 0x1e, G: 0x90, B: 0xff, A: 255}
	r, g, b, a := img.At(10, 25).RGBA() // left border of the blue rect
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != wantBlue {
		t.Errorf("first annotation's border = %v, want its own color %v", got, wantBlue)
	}

	wantGreen := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	r, g, b, a = img.At(60, 25).RGBA() // left border of the green rect
	got = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != wantGreen {
		t.Errorf("second annotation's border = %v, want its own color %v", got, wantGreen)
	}
}

// TestRenderAnnotationsFreehandDrawsAlongThePathNotARectangle confirms a
// freehand annotation (Points set) actually draws a stroke following its
// path, and specifically does NOT draw a rectangle outline — the two
// annotation kinds share one struct, so it's worth pinning down that the
// branch in RenderAnnotations picks the right one.
func TestRenderAnnotationsFreehandDrawsAlongThePathNotARectangle(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	src := solidTestImage(t, original)

	// A straight horizontal stroke from (10,50) to (80,50).
	ann := Annotation{Number: 1, X: 10, Y: 50, W: 70, H: 0, Points: []Point{{X: 10, Y: 50}, {X: 80, Y: 50}}}
	edited, err := RenderAnnotations(src, []Annotation{ann})
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	img := decodePNG(t, edited)

	// The middle of the stroke's path should be painted.
	r, g, b, a := img.At(45, 50).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got == original {
		t.Errorf("pixel (45,50) on the stroke's path = %v, want it painted (differ from untouched original)", got)
	}

	// A point well below the stroke's straight horizontal line — inside
	// what a *rectangle* outline for the same bounding box would have
	// drawn as its bottom border, but nowhere near the actual path —
	// must stay untouched. This is what actually distinguishes "drew a
	// stroke" from "drew a rectangle and got lucky."
	r, g, b, a = img.At(45, 50+int(annotationBorderThickness)+5).RGBA()
	got = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != original {
		t.Errorf("pixel below the stroke's path = %v, want untouched original %v (a rectangle would have drawn a border there; a freehand stroke should not)", got, original)
	}
}

func TestDrawPolylineSinglePointDrawsADot(t *testing.T) {
	original := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: original}, image.Point{}, draw.Src)

	c := color.RGBA{R: 200, G: 0, B: 0, A: 255}
	drawPolyline(img, []Point{{X: 25, Y: 25}}, freehandStrokeThickness, c)

	r, g, b, a := img.At(25, 25).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != c {
		t.Errorf("drawPolyline with a single point at its center = %v, want the stroke color %v", got, c)
	}
}

func TestParseAnnotationColor(t *testing.T) {
	cases := []struct {
		name string
		hex  string
		want color.RGBA
	}{
		{"empty falls back to default", "", highlightColor},
		{"with hash", "#00ff00", color.RGBA{R: 0, G: 255, B: 0, A: 255}},
		{"without hash", "00ff00", color.RGBA{R: 0, G: 255, B: 0, A: 255}},
		{"uppercase", "#00FF00", color.RGBA{R: 0, G: 255, B: 0, A: 255}},
		{"too short falls back", "#fff", highlightColor},
		{"too long falls back", "#00ff0000", highlightColor},
		{"non-hex garbage falls back", "not-a-color", highlightColor},
		{"javascript-injection-shaped string falls back", "javascript:alert(1)", highlightColor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseAnnotationColor(tc.hex); got != tc.want {
				t.Errorf("parseAnnotationColor(%q) = %v, want %v", tc.hex, got, tc.want)
			}
		})
	}
}

func TestCropReturnsOnlyTheRequestedRegion(t *testing.T) {
	// Four quadrants of distinct colors, so cropping to exactly one
	// quadrant is verifiable by checking every corner of the result.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	quadrant := func(x, y int) color.RGBA {
		switch {
		case x < 50 && y < 50:
			return color.RGBA{R: 255, A: 255} // top-left: red
		case x >= 50 && y < 50:
			return color.RGBA{G: 255, A: 255} // top-right: green
		case x < 50 && y >= 50:
			return color.RGBA{B: 255, A: 255} // bottom-left: blue
		default:
			return color.RGBA{R: 255, G: 255, A: 255} // bottom-right: yellow
		}
	}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, quadrant(x, y))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}

	// Crop to exactly the top-left (red) quadrant.
	cropped, err := Crop(buf.Bytes(), 0, 0, 50, 50)
	if err != nil {
		t.Fatalf("Crop: %v", err)
	}
	out := decodePNG(t, cropped)
	if b := out.Bounds(); b.Dx() != 50 || b.Dy() != 50 {
		t.Fatalf("cropped image size = %dx%d, want 50x50", b.Dx(), b.Dy())
	}
	for _, p := range []image.Point{{0, 0}, {49, 0}, {0, 49}, {49, 49}, {25, 25}} {
		r, g, b, a := out.At(p.X, p.Y).RGBA()
		got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
		want := color.RGBA{R: 255, A: 255}
		if got != want {
			t.Errorf("cropped pixel %v = %v, want %v (red — the crop should contain only the top-left quadrant)", p, got, want)
		}
	}
}

// TestCropClampsToImageBounds confirms a crop rectangle that extends
// past an edge (a very plausible off-by-a-few-pixels drag near a corner)
// clips to what actually exists in the image instead of erroring or
// padding the result with blank pixels.
func TestCropClampsToImageBounds(t *testing.T) {
	src := solidTestImage(t, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	cropped, err := Crop(src, 80, 80, 50, 50) // extends 30px past both edges of a 100x100 image
	if err != nil {
		t.Fatalf("Crop: %v", err)
	}
	out := decodePNG(t, cropped)
	if b := out.Bounds(); b.Dx() != 20 || b.Dy() != 20 {
		t.Errorf("clamped crop size = %dx%d, want 20x20 (100-80 in each dimension)", b.Dx(), b.Dy())
	}
}

func TestCropEntirelyOutsideImageReturnsError(t *testing.T) {
	src := solidTestImage(t, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	if _, err := Crop(src, 200, 200, 50, 50); err == nil {
		t.Error("Crop with a rectangle entirely outside the image succeeded, want an error")
	}
}

// TestRenderAnnotationsCircleDrawsAnEllipseNotARectangle confirms a
// "circle" kind actually strokes an ellipse — specifically that a pixel
// at the CORNER of the shape's bounding box (which a rectangle border
// would touch, but an ellipse inscribed in that box never reaches)
// stays untouched, while a pixel on the ellipse's actual boundary (the
// top-center point, at bbox-center-x, bbox-min-y) is painted. This is
// the same "prove it's the right shape, not just that something
// changed" standard TestRenderAnnotationsFreehandDrawsAlongThePathNotARectangle
// already set for freehand vs. rectangle.
func TestRenderAnnotationsCircleDrawsAnEllipseNotARectangle(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	src := solidTestImage(t, original)

	// Bounding box [10,90) x [10,90) — center (50,50), radii (40,40).
	ann := Annotation{X: 10, Y: 10, W: 80, H: 80, Kind: "circle"}
	edited, err := RenderAnnotations(src, []Annotation{ann})
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	img := decodePNG(t, edited)

	cr, cg, cb, ca := img.At(10, 10).RGBA()
	corner := color.RGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: uint8(ca >> 8)}
	if corner != original {
		t.Errorf("bbox corner (10,10) = %v, want untouched original %v — a rectangle border would have painted this corner, an ellipse should not", corner, original)
	}

	tr, tg, tb, ta := img.At(50, 10).RGBA()
	top := color.RGBA{R: uint8(tr >> 8), G: uint8(tg >> 8), B: uint8(tb >> 8), A: uint8(ta >> 8)}
	if top != highlightColor {
		t.Errorf("ellipse top-center (50,10) = %v, want highlight color %v — the ellipse boundary should pass through here", top, highlightColor)
	}
}

// TestRenderAnnotationsLineDrawsOnlyTheSegment confirms a "line"
// annotation strokes the straight segment between its two points and
// nothing else — checked by confirming a point ON the segment is
// painted and a point well off the segment (but inside the segment's
// own bounding box) is not, the same "not a rectangle" distinction
// freehand's own test makes.
func TestRenderAnnotationsLineDrawsOnlyTheSegment(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	src := solidTestImage(t, original)

	// Diagonal from (10,10) to (90,90) — its own bounding box is the
	// whole 80x80 square, but the stroke itself should only touch
	// pixels near the diagonal.
	ann := Annotation{X: 10, Y: 10, W: 80, H: 80, Kind: "line", Points: []Point{{X: 10, Y: 10}, {X: 90, Y: 90}}}
	edited, err := RenderAnnotations(src, []Annotation{ann})
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	img := decodePNG(t, edited)

	mr, mg, mb, ma := img.At(50, 50).RGBA()
	mid := color.RGBA{R: uint8(mr >> 8), G: uint8(mg >> 8), B: uint8(mb >> 8), A: uint8(ma >> 8)}
	if mid != highlightColor {
		t.Errorf("line midpoint (50,50) = %v, want highlight color %v", mid, highlightColor)
	}

	fr, fg, fb, fa := img.At(85, 15).RGBA() // near a corner of the bbox, far from the diagonal
	farFromLine := color.RGBA{R: uint8(fr >> 8), G: uint8(fg >> 8), B: uint8(fb >> 8), A: uint8(fa >> 8)}
	if farFromLine != original {
		t.Errorf("point (85,15), far from the diagonal but inside its bbox, = %v, want untouched original %v — a rectangle border would paint here, a line should not", farFromLine, original)
	}
}

// TestRenderAnnotationsArrowDrawsAnArrowheadAtTheSecondPoint confirms
// an "arrow" annotation draws extra pixels near its endpoint (the
// arrowhead's wings) beyond what a plain "line" of the same two points
// would — the one visible difference between the two kinds.
func TestRenderAnnotationsArrowDrawsAnArrowheadAtTheSecondPoint(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	points := []Point{{X: 10, Y: 50}, {X: 80, Y: 50}}

	lineOnly, err := RenderAnnotations(solidTestImage(t, original), []Annotation{{X: 10, Y: 50, W: 70, H: 0, Kind: "line", Points: points}})
	if err != nil {
		t.Fatalf("RenderAnnotations (line): %v", err)
	}
	withArrow, err := RenderAnnotations(solidTestImage(t, original), []Annotation{{X: 10, Y: 50, W: 70, H: 0, Kind: "arrow", Points: points}})
	if err != nil {
		t.Fatalf("RenderAnnotations (arrow): %v", err)
	}
	lineImg := decodePNG(t, lineOnly)
	arrowImg := decodePNG(t, withArrow)

	// A point on one of the arrowhead's wings (computed from the same
	// geometry drawArrowhead uses: wingLength 14 at ~25.7 degrees off
	// the shaft, 60% of the way from the tip toward the wing's own
	// endpoint) — 4px below the shaft's centerline, safely outside the
	// bare shaft's own ~2px stroke half-width.
	const probeX, probeY = 72, 54
	lr, lg, lb, la := lineImg.At(probeX, probeY).RGBA()
	lineWing := color.RGBA{R: uint8(lr >> 8), G: uint8(lg >> 8), B: uint8(lb >> 8), A: uint8(la >> 8)}
	ar, ag, ab, aa := arrowImg.At(probeX, probeY).RGBA()
	arrowWing := color.RGBA{R: uint8(ar >> 8), G: uint8(ag >> 8), B: uint8(ab >> 8), A: uint8(aa >> 8)}

	if lineWing == highlightColor {
		t.Fatalf("test assumption broken: a plain line already paints (%d,%d) — pick a different probe point that only the arrowhead reaches", probeX, probeY)
	}
	if arrowWing != highlightColor {
		t.Errorf("arrow's wing area (%d,%d) = %v, want highlight color %v — the arrowhead should extend beyond the bare shaft", probeX, probeY, arrowWing, highlightColor)
	}
}

// TestRenderAnnotationsAnnotatedFalseSuppressesBadge confirms decision
// #30's default: a shape with Annotated: false draws its outline but no
// numbered badge at all — the corner where the badge would otherwise
// sit (the shape's own X,Y) stays untouched.
func TestRenderAnnotationsAnnotatedFalseSuppressesBadge(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	src := solidTestImage(t, original)

	ann := Annotation{Number: 3, X: 20, Y: 20, W: 50, H: 50, Annotated: false}
	edited, err := RenderAnnotations(src, []Annotation{ann})
	if err != nil {
		t.Fatalf("RenderAnnotations: %v", err)
	}
	img := decodePNG(t, edited)

	// The border should still be there (Annotated only gates the badge).
	br, bg, bb, ba := img.At(20, 45).RGBA()
	border := color.RGBA{R: uint8(br >> 8), G: uint8(bg >> 8), B: uint8(bb >> 8), A: uint8(ba >> 8)}
	if border != highlightColor {
		t.Errorf("border with Annotated: false = %v, want it still drawn (highlight color)", border)
	}

	// The badge (a filled disc at the shape's X,Y) should NOT be there —
	// check the exact center point, which the badge would fill solid.
	cr, cg, cb, ca := img.At(20, 20).RGBA()
	center := color.RGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: uint8(ca >> 8)}
	if center == badgeTextColor || (center.R > original.R && center != highlightColor) {
		t.Errorf("badge area (20,20) with Annotated: false = %v, want no badge drawn there at all", center)
	}
}

// TestShapeKindLegacyPointsWithNoKindIsFreehand confirms data saved
// before the Kind field existed — Kind == "" with non-empty Points, the
// only thing that combination could ever have meant pre-decision-#30 —
// still resolves to "freehand", not the "rect" fallback that empty Kind
// would otherwise mean for a box shape with no Points.
func TestShapeKindLegacyPointsWithNoKindIsFreehand(t *testing.T) {
	legacy := Annotation{Points: []Point{{X: 0, Y: 0}, {X: 10, Y: 10}}}
	if got := shapeKind(legacy); got != "freehand" {
		t.Errorf("shapeKind(legacy Points, no Kind) = %q, want %q", got, "freehand")
	}
	plainRect := Annotation{}
	if got := shapeKind(plainRect); got != "rect" {
		t.Errorf("shapeKind(no Points, no Kind) = %q, want %q", got, "rect")
	}
}
