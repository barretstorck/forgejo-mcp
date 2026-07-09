package extract

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// minTextLayerChars: pages whose text layer yields fewer non-space chars
// are treated as scans and OCR'd.
const minTextLayerChars = 16

func needsOCR(pageText string) bool {
	return len(strings.TrimSpace(pageText)) < minTextLayerChars
}

// RenderPDFPagePNG renders one page (1-based) to PNG. maxEdge > 0 scales the
// longest edge to that many pixels; maxEdge == 0 renders at 200 dpi (OCR use).
func (e *Extractor) RenderPDFPagePNG(ctx context.Context, data []byte, page, maxEdge int) ([]byte, error) {
	if e.PdftoppmPath == "" {
		return nil, fmt.Errorf("pdftoppm is not installed in this container")
	}
	tmp, err := writeTemp(data, "*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	dir, err := os.MkdirTemp("", "forgejo-mcp-ppm")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := dir + "/page"

	args := []string{"-png", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page)}
	if maxEdge > 0 {
		args = append(args, "-scale-to", strconv.Itoa(maxEdge))
	} else {
		args = append(args, "-r", "200")
	}
	args = append(args, tmp, prefix)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, e.PdftoppmPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}

	matches, _ := filepath.Glob(prefix + "*.png")
	if len(matches) == 0 {
		return nil, fmt.Errorf("pdftoppm produced no output for page %d (page out of range?)", page)
	}
	return os.ReadFile(matches[0])
}

// ocrPNG runs tesseract on PNG bytes and returns the recognized text.
func (e *Extractor) ocrPNG(ctx context.Context, png []byte) (string, error) {
	if e.TesseractPath == "" {
		return "", fmt.Errorf("tesseract is not installed in this container")
	}

	// tesseract's "stdin stdout" mode produced empty output in the alpine
	// test container, so OCR reads from a temp file instead (same pattern
	// as writeTemp elsewhere in this package).
	tmp, err := writeTemp(png, "*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, e.TesseractPath, tmp, "stdout", "-l", "eng")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// ExtractImageText OCRs a standalone image (one-page document).
func (e *Extractor) ExtractImageText(ctx context.Context, data []byte) ([]PageText, error) {
	text, err := e.ocrPNG(ctx, data)
	if err != nil {
		return nil, err
	}
	return []PageText{{Page: 1, Text: text}}, nil
}

// ExtractPDFTextOCR extracts the text layer and OCRs low-yield (scanned) pages.
func (e *Extractor) ExtractPDFTextOCR(ctx context.Context, data []byte) ([]PageText, error) {
	pages, err := e.ExtractPDFText(ctx, data)
	if err != nil {
		return nil, err
	}
	for i := range pages {
		if !needsOCR(pages[i].Text) {
			continue
		}
		png, rerr := e.RenderPDFPagePNG(ctx, data, pages[i].Page, 0)
		if rerr != nil {
			continue // keep the (blank) text layer; rendering failure is not fatal
		}
		if text, oerr := e.ocrPNG(ctx, png); oerr == nil && text != "" {
			pages[i].Text = text
		}
	}
	return pages, nil
}
