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

	ann := Annotation{Number: 7, X: 20, Y: 20, W: 50, H: 50} // rect [20,70) x [20,70)
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
