package extract

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExtractPDFText extracts the embedded text layer, one PageText per page.
// pdftotext emits form-feed (\f) between pages.
func (e *Extractor) ExtractPDFText(ctx context.Context, data []byte) ([]PageText, error) {
	if e.PdftotextPath == "" {
		return nil, fmt.Errorf("pdftotext is not installed in this container")
	}
	tmp, err := writeTemp(data, "*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, e.PdftotextPath, "-layout", tmp, "-")
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	raw := strings.Split(strings.TrimSuffix(out.String(), "\f"), "\f")
	pages := make([]PageText, len(raw))
	for i, text := range raw {
		pages[i] = PageText{Page: i + 1, Text: strings.TrimSpace(text)}
	}
	return pages, nil
}

// writeTemp writes data to a temp file matching pattern and returns its path.
func writeTemp(data []byte, pattern string) (string, error) {
	f, err := os.CreateTemp("", "forgejo-mcp-"+pattern)
	if err != nil {
		return "", fmt.Errorf("create temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp file: %v", err)
	}
	f.Close()
	p, _ := filepath.Abs(f.Name())
	return p, nil
}
