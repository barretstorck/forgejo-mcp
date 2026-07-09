package extract

import (
	"context"
	"strings"
	"testing"
)

func TestExtractPDFText_TwoPages(t *testing.T) {
	e := New()
	requireBinary(t, e.PdftotextPath, "pdftotext")
	pdf := buildFixturePDF([]string{"alpha token page one", "bravo token page two"})
	pages, err := e.ExtractPDFText(context.Background(), pdf)
	if err != nil {
		t.Fatalf("ExtractPDFText: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	if pages[0].Page != 1 || !strings.Contains(pages[0].Text, "alpha token") {
		t.Fatalf("page 1 = %+v", pages[0])
	}
	if pages[1].Page != 2 || !strings.Contains(pages[1].Text, "bravo token") {
		t.Fatalf("page 2 = %+v", pages[1])
	}
}
