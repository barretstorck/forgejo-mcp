package extract

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestExtractDOCXText(t *testing.T) {
	docx := zipFixture(t, map[string]string{
		"word/document.xml": `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body><w:p><w:r><w:t>echo token one</w:t></w:r></w:p>
<w:p><w:r><w:t xml:space="preserve">foxtrot token two</w:t></w:r></w:p></w:body></w:document>`,
	})
	pages, err := ExtractDOCXText(docx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || !strings.Contains(pages[0].Text, "echo token one") || !strings.Contains(pages[0].Text, "foxtrot token two") {
		t.Fatalf("pages = %+v", pages)
	}
}

func TestExtractXLSXText(t *testing.T) {
	xlsx := zipFixture(t, map[string]string{
		"xl/sharedStrings.xml": `<?xml version="1.0"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2" uniqueCount="2">
<si><t>golf token</t></si><si><r><t>hotel </t></r><r><t>token</t></r></si></sst>`,
	})
	pages, err := ExtractXLSXText(xlsx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || !strings.Contains(pages[0].Text, "golf token") || !strings.Contains(pages[0].Text, "hotel token") {
		t.Fatalf("pages = %+v", pages)
	}
}

func TestExtractDOCXText_NotAZip(t *testing.T) {
	if _, err := ExtractDOCXText([]byte("plain text")); err == nil {
		t.Fatal("want error for non-zip input")
	}
}
