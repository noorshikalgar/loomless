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

	edited, err := Redact(src, 20, 20, 30, 30) // rect [20,50) x [20,50)
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

func TestDrawBoxHighlightOutlinesOnlyLeavesInteriorUntouched(t *testing.T) {
	original := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	src := solidTestImage(t, original)

	edited, err := DrawBoxHighlight(src, 20, 20, 40, 40) // rect [20,60) x [20,60)
	if err != nil {
		t.Fatalf("DrawBoxHighlight: %v", err)
	}
	img := decodePNG(t, edited)

	// Center of the box should NOT be painted — only a border, unlike
	// Redact which fills solid. If this fails, a highlight would be
	// indistinguishable from a redaction, defeating the point of having
	// both (marking something up vs. hiding it).
	r, g, b, a := img.At(40, 40).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != original {
		t.Errorf("center pixel (40,40) = %v, want untouched original %v (highlight should outline, not fill)", got, original)
	}

	// The border itself should be the highlight color, not the original.
	r, g, b, a = img.At(20, 40).RGBA()
	got = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got == original {
		t.Errorf("border pixel (20,40) = %v, want highlight color, not unchanged original", got)
	}
}
