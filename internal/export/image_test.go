package export

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/noorshikalgar/story/internal/session"
)

// writeTestPNG creates a 100x100 solid-color PNG at dir/name, returning
// the relative filename the way session.Step.ImageFile stores it.
func writeTestPNG(t *testing.T, dir, name string, fill color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test image: %v", err)
	}
	return name
}

func TestStepImageBytesAppliesCropBeforeAnnotations(t *testing.T) {
	dir := t.TempDir()
	imageFile := writeTestPNG(t, dir, "step-0001.png", color.RGBA{R: 10, G: 20, B: 30, A: 255})

	step := session.Step{
		Index:     1,
		ImageFile: imageFile,
		Crop:      &session.CropRect{X: 20, Y: 20, W: 40, H: 40}, // keep [20,60) x [20,60)
		Annotations: []session.Annotation{
			// In ORIGINAL image coordinates, this sits at the crop's
			// top-left corner — after the shift it should land at (0,0)
			// in the cropped canvas, not (30,30).
			{ID: 1, Number: 1, X: 20, Y: 20, W: 10, H: 10},
		},
	}

	out, err := stepImageBytes(dir, step)
	if err != nil {
		t.Fatalf("stepImageBytes: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Fatalf("cropped+annotated image size = %dx%d, want 40x40", b.Dx(), b.Dy())
	}

	// The annotation's border should now be drawn right at the cropped
	// canvas's own top-left corner (0,0), proving the coordinate shift
	// happened — if it hadn't, the annotation would either be miss-drawn
	// far outside the 40x40 canvas (invisible) or at the wrong spot.
	r, g, b, a := img.At(0, 0).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got == (color.RGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Error("pixel (0,0) of the cropped+annotated image is still the untouched background — the annotation didn't shift into the cropped canvas's coordinate space")
	}
}

func TestStepImageBytesCropWithNoAnnotations(t *testing.T) {
	dir := t.TempDir()
	imageFile := writeTestPNG(t, dir, "step-0001.png", color.RGBA{R: 1, G: 2, B: 3, A: 255})
	step := session.Step{Index: 1, ImageFile: imageFile, Crop: &session.CropRect{X: 10, Y: 10, W: 30, H: 30}}

	out, err := stepImageBytes(dir, step)
	if err != nil {
		t.Fatalf("stepImageBytes: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 30 || b.Dy() != 30 {
		t.Errorf("cropped image size = %dx%d, want 30x30", b.Dx(), b.Dy())
	}
}

func TestStepImageBytesNoCropNoAnnotationsReturnsOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	imageFile := writeTestPNG(t, dir, "step-0001.png", color.RGBA{R: 5, G: 6, B: 7, A: 255})
	original, err := os.ReadFile(filepath.Join(dir, imageFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	out, err := stepImageBytes(dir, session.Step{Index: 1, ImageFile: imageFile})
	if err != nil {
		t.Fatalf("stepImageBytes: %v", err)
	}
	if !bytes.Equal(original, out) {
		t.Error("stepImageBytes with no crop and no annotations should return the original bytes unchanged")
	}
}

func TestShiftAnnotationsForCropDoesNotMutateInput(t *testing.T) {
	original := []session.Annotation{{ID: 1, Number: 1, X: 50, Y: 50, W: 10, H: 10}}
	shiftAnnotationsForCrop(original, session.CropRect{X: 20, Y: 20, W: 40, H: 40})

	if original[0].X != 50 || original[0].Y != 50 {
		t.Errorf("shiftAnnotationsForCrop mutated the caller's slice: got X=%v Y=%v, want unchanged 50,50", original[0].X, original[0].Y)
	}
}

func TestStepImagePathWritesFlattenedCopyForCropAlone(t *testing.T) {
	dir := t.TempDir()
	imageFile := writeTestPNG(t, dir, "step-0001.png", color.RGBA{R: 1, G: 2, B: 3, A: 255})
	step := session.Step{Index: 1, ImageFile: imageFile, Crop: &session.CropRect{X: 5, Y: 5, W: 50, H: 50}}

	path, err := stepImagePath(dir, step)
	if err != nil {
		t.Fatalf("stepImagePath: %v", err)
	}
	if path == imageFile {
		t.Fatal("stepImagePath returned the original (uncropped) file path — a crop with no annotations must still produce a flattened copy")
	}
	data, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatalf("read flattened copy: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode flattened copy: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 50 || b.Dy() != 50 {
		t.Errorf("flattened copy size = %dx%d, want 50x50 (the crop)", b.Dx(), b.Dy())
	}
}
