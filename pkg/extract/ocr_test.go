package extract

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// rasterizedFixture returns a PNG render of a 1-page fixture PDF — a
// synthetic "scanned document" (no text layer) produced by the same
// poppler binary the OCR path depends on.
func rasterizedFixture(t *testing.T, text string) []byte {
	t.Helper()
	e := New()
	requireBinary(t, e.PdftoppmPath, "pdftoppm")
	pdf := buildFixturePDF([]string{text})
	png, err := e.RenderPDFPagePNG(context.Background(), pdf, 1, 0)
	if err != nil {
		t.Fatalf("RenderPDFPagePNG: %v", err)
	}
	return png
}

func TestExtractImageText_OCR(t *testing.T) {
	e := New()
	requireBinary(t, e.TesseractPath, "tesseract")
	png := rasterizedFixture(t, "CHARLIE TOKEN 12345")
	pages, err := e.ExtractImageText(context.Background(), png)
	if err != nil {
		t.Fatalf("ExtractImageText: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	up := strings.ToUpper(pages[0].Text)
	if !strings.Contains(up, "CHARLIE") || !strings.Contains(up, "12345") {
		t.Fatalf("OCR text %q missing expected tokens", pages[0].Text)
	}
}

func TestRenderPDFPagePNG_ProducesPNG(t *testing.T) {
	png := rasterizedFixture(t, "delta token")
	if !bytes.HasPrefix(png, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("output is not a PNG")
	}
}

func TestNeedsOCR(t *testing.T) {
	if !needsOCR("   \n ") {
		t.Fatal("blank page must need OCR")
	}
	if needsOCR("This page has a perfectly good text layer.") {
		t.Fatal("normal text must not need OCR")
	}
}
