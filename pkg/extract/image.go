package extract

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// NormalizeImagePNG decodes a PNG/JPEG/WebP image, downscales it so its
// longest edge is at most maxEdge pixels (when larger), and returns PNG.
func NormalizeImagePNG(data []byte, maxEdge int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxEdge > 0 && (w > maxEdge || h > maxEdge) {
		scale := float64(maxEdge) / float64(w)
		if h > w {
			scale = float64(maxEdge) / float64(h)
		}
		nw, nh := int(float64(w)*scale), int(float64(h)*scale)
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
		src = dst
	}
	var out bytes.Buffer
	if err := png.Encode(&out, src); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return out.Bytes(), nil
}
