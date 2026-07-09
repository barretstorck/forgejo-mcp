package extract

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestNormalizeImagePNG_Downscales(t *testing.T) {
	out, err := NormalizeImagePNG(makePNG(t, 4000, 2000), 1000)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1000 || img.Bounds().Dy() != 500 {
		t.Fatalf("got %v, want 1000x500", img.Bounds())
	}
}

func TestNormalizeImagePNG_SmallPassthroughDimensions(t *testing.T) {
	out, err := NormalizeImagePNG(makePNG(t, 300, 200), 1280)
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(out))
	if img.Bounds().Dx() != 300 || img.Bounds().Dy() != 200 {
		t.Fatalf("small image must keep its size, got %v", img.Bounds())
	}
}

// TestNormalizeImagePNG_ExtremeAspectRatioFloorsToOnePixel guards against a
// zero-dimension scaled output. A 2000x2 image normalized at maxEdge=500
// scales the short edge to 500*(2/2000)=0.5, which must floor to 1 (not 0):
// an RGBA image with a 0 dimension produces a malformed/undecodable PNG.
func TestNormalizeImagePNG_ExtremeAspectRatioFloorsToOnePixel(t *testing.T) {
	out, err := NormalizeImagePNG(makePNG(t, 2000, 2), 500)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result png: %v", err)
	}
	if img.Bounds().Dx() != 500 || img.Bounds().Dy() != 1 {
		t.Fatalf("got %v, want 500x1", img.Bounds())
	}
}

func TestNormalizeImagePNG_JPEGDecode(t *testing.T) {
	out, err := NormalizeImagePNG(makeJPEG(t, 4000, 2000), 1000)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result png: %v", err)
	}
	if img.Bounds().Dx() != 1000 || img.Bounds().Dy() != 500 {
		t.Fatalf("got %v, want 1000x500", img.Bounds())
	}
}
