// Package extract turns repository documents (PDF, images, OOXML) into
// per-page text and page images using CLI tools (poppler, tesseract) and
// pure-Go OOXML parsing. All output is size-conscious: callers cap what
// they return to LLM clients.
package extract

import "os/exec"

// Version participates in cache keys; bump when extraction output changes.
const Version = "v1"

type PageText struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

type Extractor struct {
	PdftotextPath string
	PdftoppmPath  string
	TesseractPath string
}

// New resolves the CLI dependencies. Missing binaries leave empty paths;
// the corresponding operations return descriptive errors when attempted.
func New() *Extractor {
	e := &Extractor{}
	if p, err := exec.LookPath("pdftotext"); err == nil {
		e.PdftotextPath = p
	}
	if p, err := exec.LookPath("pdftoppm"); err == nil {
		e.PdftoppmPath = p
	}
	if p, err := exec.LookPath("tesseract"); err == nil {
		e.TesseractPath = p
	}
	return e
}
