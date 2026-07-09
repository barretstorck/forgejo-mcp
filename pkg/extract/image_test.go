package extract

import (
	"bytes"
	"image"
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
	if img.Bounds().Dx() != 300 {
		t.Fatalf("small image must keep its size, got %v", img.Bounds())
	}
}
